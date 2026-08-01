#!/usr/bin/env python3
"""
segment.py — one pass over a PDF: text, outline, and layout regions.

Aladin's ingestion worker shells out to this once per document
(design/INGESTION_PRD.md §13). Contract, kept deliberately dumb:

    in   : --pdf PATH
    out  : one JSON document on stdout
    fail : diagnostics on stderr, non-zero exit

Why text AND regions come from the same pass: a region's bounding box has to resolve
to the page and words underneath it. Extracting text with one library and boxes with
another makes that an inference across two coordinate models. §13d puts the whole error
budget on boundaries and none on anchors — a wrong page is a confident false citation —
so anchoring is done here, once, where the coordinates are known to agree.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys

EXTRACTOR = "pdf/pymupdf+doclayout-yolo@1"

# Measured on the corpus (INGESTION_PRD §13c): below this the boxes are mostly junk.
MIN_CONFIDENCE = 0.30
# Two boxes overlapping more than this are the same thing seen twice.
IOU_THRESHOLD = 0.60
# "BookRe" — a fragment, not a heading.
MIN_TITLE_CHARS = 4
# Layout runs at this DPI: the accuracy floor. 96 loses 23% of regions, 200 buys nothing.
RENDER_DPI = 144
MODEL_IMGSZ = 1024

MODEL_REPO = "juliozhao/DocLayout-YOLO-DocStructBench"
MODEL_FILE = "doclayout_yolo_docstructbench_imgsz1024.pt"

# The ACM paper labelled "Latest updates: https://dl.acm.org/doi/…" a title.
URL_RE = re.compile(r"https?://|www\.|doi\.org|\bdl\.acm\.org\b", re.I)


def die(message: str, code: int = 1) -> None:
    print(message, file=sys.stderr)
    sys.exit(code)


def pick_device(requested: str) -> str:
    """mps where it exists, cpu otherwise. Never hardcode — mps doesn't exist in a
    Linux container, and the Metal numbers don't transfer anyway (§13b)."""
    if requested != "auto":
        return requested
    try:
        import torch

        if torch.backends.mps.is_available():
            return "mps"
        if torch.cuda.is_available():
            return "cuda"
    except Exception:
        pass
    return "cpu"


def iou(a: list[float], b: list[float]) -> float:
    ax0, ay0, ax1, ay1 = a
    bx0, by0, bx1, by1 = b
    ix0, iy0 = max(ax0, bx0), max(ay0, by0)
    ix1, iy1 = min(ax1, bx1), min(ay1, by1)
    if ix1 <= ix0 or iy1 <= iy0:
        return 0.0
    inter = (ix1 - ix0) * (iy1 - iy0)
    area_a = max(0.0, ax1 - ax0) * max(0.0, ay1 - ay0)
    area_b = max(0.0, bx1 - bx0) * max(0.0, by1 - by0)
    union = area_a + area_b - inter
    return inter / union if union > 0 else 0.0


def clean_regions(regions: list[dict]) -> list[dict]:
    """Apply the filters the corpus check earned (§13c).

    Overlap dedup runs across classes, not within: the duplicates observed were the same
    text boxed twice under different labels, which a per-class NMS never sees.
    """
    kept: list[dict] = []
    for region in sorted(regions, key=lambda r: -r["confidence"]):
        text = (region.get("text") or "").strip()

        if region["class"] == "title":
            # A DOI line is page furniture wearing a heading's clothes.
            if URL_RE.search(text):
                region["class"] = "abandon"
            elif 0 < len(text) < MIN_TITLE_CHARS:
                continue  # a fragment is worse than nothing: it would split a chunk

        if any(iou(region["bbox"], other["bbox"]) > IOU_THRESHOLD for other in kept):
            continue
        kept.append(region)

    # Reading order, roughly: top to bottom, then left to right.
    kept.sort(key=lambda r: (round(r["bbox"][1], 1), r["bbox"][0]))
    return kept


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pdf", required=True)
    parser.add_argument("--device", default="auto", help="auto | mps | cuda | cpu")
    parser.add_argument("--dpi", type=int, default=RENDER_DPI)
    parser.add_argument("--max-pages", type=int, default=0, help="0 = all")
    parser.add_argument("--no-layout", action="store_true",
                        help="text + outline only; skips loading the model")
    args = parser.parse_args()

    if not os.path.isfile(args.pdf):
        die(f"no such file: {args.pdf}")

    try:
        import fitz  # PyMuPDF
    except ImportError as e:
        die(f"PyMuPDF missing — is the venv installed? ({e})")

    try:
        doc = fitz.open(args.pdf)
    except Exception as e:
        die(f"cannot open PDF: {e}")

    if doc.needs_pass:
        die("PDF is password-protected")

    model = None
    device = "none"
    if not args.no_layout:
        try:
            from doclayout_yolo import YOLOv10
            from huggingface_hub import hf_hub_download

            device = pick_device(args.device)
            model = YOLOv10(hf_hub_download(MODEL_REPO, MODEL_FILE))
        except Exception as e:
            # Layout is an enhancement; text alone is still a usable document. Report the
            # reason and carry on rather than failing the whole ingest.
            print(f"layout model unavailable, continuing text-only: {e}", file=sys.stderr)
            model = None

    # The document's own outline, when it ships one. §5: never invent structure.
    outline = []
    try:
        for level, title, page in doc.get_toc(simple=True):
            title = (title or "").strip()
            if title and page > 0:
                outline.append({"title": title, "level": max(0, level - 1), "page": page})
    except Exception:
        pass

    total = len(doc)
    limit = min(total, args.max_pages) if args.max_pages else total
    pages = []
    import tempfile

    with tempfile.TemporaryDirectory() as tmp:
        for index in range(limit):
            page = doc[index]
            entry = {
                "page": index + 1,
                "width": round(page.rect.width, 2),
                "height": round(page.rect.height, 2),
                "text": page.get_text().strip(),
                "regions": [],
            }

            if model is not None:
                try:
                    pix = page.get_pixmap(dpi=args.dpi)
                    shot = os.path.join(tmp, f"p{index}.png")
                    pix.save(shot)
                    result = model.predict(shot, imgsz=MODEL_IMGSZ, conf=MIN_CONFIDENCE,
                                           device=device, verbose=False)[0]
                    # Boxes come back in pixel space; everything downstream speaks PDF
                    # points, and mixing the two is exactly how anchors go wrong.
                    scale = page.rect.width / pix.width
                    raw = []
                    for box in result.boxes:
                        x0, y0, x1, y1 = [float(v) * scale for v in box.xyxy[0]]
                        rect = fitz.Rect(x0, y0, x1, y1)
                        raw.append({
                            "class": result.names[int(box.cls)],
                            "confidence": round(float(box.conf), 3),
                            "bbox": [round(x0, 2), round(y0, 2), round(x1, 2), round(y1, 2)],
                            "text": page.get_textbox(rect).strip(),
                        })
                    entry["regions"] = clean_regions(raw)
                    os.unlink(shot)
                except Exception as e:
                    # One bad page must not lose the other 279.
                    print(f"page {index + 1}: layout failed: {e}", file=sys.stderr)

            pages.append(entry)

    json.dump({
        "extractor": EXTRACTOR,
        "device": device,
        "page_count": total,
        "pages_processed": len(pages),
        "outline": outline,
        "pages": pages,
    }, sys.stdout)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())

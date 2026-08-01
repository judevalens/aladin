"""Draw the model's boxes onto page images so a human can judge them.

Numbers like "28% of regions were titles" are unfalsifiable without seeing where the
boxes actually landed.
"""
import sys, os
import fitz
from PIL import Image, ImageDraw, ImageFont
from huggingface_hub import hf_hub_download
from doclayout_yolo import YOLOv10

COLORS = {
    "title":            (220, 50, 50),     # red    — the ones that matter
    "plain text":       (70, 130, 200),    # blue
    "abandon":          (150, 150, 150),   # grey   — page furniture, ignored
    "figure":           (60, 170, 110),    # green
    "figure_caption":   (60, 170, 110),
    "table":            (200, 140, 40),    # amber
    "table_caption":    (200, 140, 40),
    "table_footnote":   (200, 140, 40),
    "isolate_formula":  (150, 90, 200),    # violet
    "formula_caption":  (150, 90, 200),
}

weights = hf_hub_download("juliozhao/DocLayout-YOLO-DocStructBench",
                          "doclayout_yolo_docstructbench_imgsz1024.pt")
model = YOLOv10(weights)
outdir = "out"; os.makedirs(outdir, exist_ok=True)
try:
    font = ImageFont.truetype("/System/Library/Fonts/Supplemental/Arial Bold.ttf", 20)
except Exception:
    font = ImageFont.load_default()

for spec in sys.argv[1:]:
    path, pages = spec.split("::")
    doc = fitz.open(path)
    stem = os.path.basename(path)[:22].replace(" ", "_")
    for pno in [int(p) for p in pages.split(",")]:
        page = doc[pno - 1]
        pix = page.get_pixmap(dpi=144)
        raw = f"/tmp/raw.png"; pix.save(raw)
        res = model.predict(raw, imgsz=1024, conf=0.3, device="mps", verbose=False)[0]

        img = Image.open(raw).convert("RGB")
        d = ImageDraw.Draw(img, "RGBA")
        counts = {}
        for b in res.boxes:
            label = res.names[int(b.cls)]
            conf = float(b.conf)
            counts[label] = counts.get(label, 0) + 1
            x0, y0, x1, y1 = [float(v) for v in b.xyxy[0]]
            c = COLORS.get(label, (0, 0, 0))
            width = 5 if label == "title" else 2
            d.rectangle([x0, y0, x1, y1], outline=c, width=width)
            tag = f"{label} {conf:.2f}"
            tw = d.textlength(tag, font=font)
            d.rectangle([x0, max(0, y0 - 26), x0 + tw + 10, y0], fill=c + (235,))
            d.text((x0 + 5, max(0, y0 - 24)), tag, fill=(255, 255, 255), font=font)

        banner = f"{stem}  p{pno}   " + "  ".join(f"{k}:{v}" for k, v in sorted(counts.items()))
        d.rectangle([0, 0, img.width, 34], fill=(20, 20, 24, 240))
        d.text((10, 7), banner, fill=(255, 255, 255), font=font)

        out = f"{outdir}/{stem}_p{pno}.png"
        img.save(out)
        print(out)

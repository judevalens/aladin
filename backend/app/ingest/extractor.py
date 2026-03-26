from pathlib import Path
import fitz  # pymupdf


def extract(path: Path) -> dict:
    """Extract text, metadata and page count from a PDF."""
    doc = fitz.open(str(path))

    meta = doc.metadata or {}
    title = _extract_title(doc, meta)

    pages = []
    for i, page in enumerate(doc):
        text = page.get_text().strip()
        if text:
            pages.append({"page": i + 1, "text": text})

    full_text = "\n\n".join(p["text"] for p in pages)

    return {
        "title": title,
        "page_count": len(doc),
        "pages": pages,
        "full_text": full_text,
        "author": meta.get("author", ""),
    }


def _extract_title(doc: fitz.Document, meta: dict) -> str | None:
    # 1. PDF metadata title
    if title := meta.get("title", "").strip():
        return title

    # 2. First non-empty line of page 1 (common in academic PDFs)
    if len(doc) > 0:
        first_line = doc[0].get_text().strip().split("\n")[0].strip()
        if len(first_line) > 10:
            return first_line

    return None

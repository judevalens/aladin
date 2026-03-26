def chunk_pages(pages: list[dict], max_size: int = 1000, overlap: int = 100) -> list[dict]:
    """
    Chunk page text into overlapping segments.
    Returns list of {page, text} dicts.
    """
    chunks = []
    for page in pages:
        text = page["text"]
        page_num = page["page"]

        if len(text) <= max_size:
            chunks.append({"page": page_num, "text": text})
            continue

        start = 0
        while start < len(text):
            end = min(start + max_size, len(text))
            chunks.append({"page": page_num, "text": text[start:end]})
            if end == len(text):
                break
            start = end - overlap

    return chunks

import hashlib
import uuid
from pathlib import Path

from app.db import get_session_factory
from app.db.models import Document, Chunk
from app.db.seed import DEFAULT_USER_ID
from app.ingest.extractor import extract
from app.ingest.chunker import chunk_pages

UPLOAD_DIR = Path(__file__).parents[2] / "uploads"
UPLOAD_DIR.mkdir(exist_ok=True)


def ingest(file_bytes: bytes, original_filename: str) -> dict:
    doc_id = f"sha256:{hashlib.sha256(file_bytes).hexdigest()}"

    db = get_session_factory()()
    try:
        # Already ingested — return existing record
        existing = db.get(Document, doc_id)
        if existing:
            return _serialize(existing, status="already_exists")

        # Save file to disk
        dest = UPLOAD_DIR / f"{doc_id}.pdf"
        dest.write_bytes(file_bytes)

        # Extract text + metadata
        extracted = extract(dest)

        # Persist document
        doc = Document(
            id=doc_id,
            filename=original_filename,
            title=extracted["title"],
            size=len(file_bytes),
            page_count=extracted["page_count"],
            uploaded_by=DEFAULT_USER_ID,
            ingested=False,
        )
        db.add(doc)
        db.flush()

        # Persist chunks
        for chunk in chunk_pages(extracted["pages"]):
            db.add(Chunk(
                id=uuid.uuid4(),
                document_id=doc_id,
                page=chunk["page"],
                text=chunk["text"],
            ))

        doc.ingested = True
        db.commit()
        db.refresh(doc)

        return _serialize(doc, status="ingested")

    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def _serialize(doc: Document, status: str) -> dict:
    return {
        "id": doc.id,
        "filename": doc.filename,
        "title": doc.title,
        "size": doc.size,
        "page_count": doc.page_count,
        "uploaded_at": doc.uploaded_at.isoformat(),
        "ingested": doc.ingested,
        "status": status,
    }

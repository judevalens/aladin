import uuid
from datetime import datetime, timezone
from sqlalchemy import Text, Integer, Float, Boolean, ForeignKey, DateTime
from sqlalchemy.dialects.postgresql import JSONB, UUID
from sqlalchemy.orm import mapped_column, Mapped, relationship
from pgvector.sqlalchemy import Vector
from . import Base


class User(Base):
    __tablename__ = "users"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    email: Mapped[str] = mapped_column(Text, unique=True, nullable=False)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    documents: Mapped[list["Document"]] = relationship(back_populates="owner")
    knowledge_graphs: Mapped[list["KnowledgeGraph"]] = relationship(back_populates="user")
    sources: Mapped[list["Source"]] = relationship(back_populates="user")


class Document(Base):
    __tablename__ = "documents"

    id: Mapped[str] = mapped_column(Text, primary_key=True)
    filename: Mapped[str] = mapped_column(Text, nullable=False)
    title: Mapped[str | None] = mapped_column(Text)
    size: Mapped[int | None] = mapped_column(Integer)
    page_count: Mapped[int | None] = mapped_column(Integer)
    uploaded_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )
    ingested: Mapped[bool] = mapped_column(Boolean, default=False)
    uploaded_by: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("users.id"), nullable=True
    )

    owner: Mapped["User | None"] = relationship(back_populates="documents")
    chunks: Mapped[list["Chunk"]] = relationship(back_populates="document")


class Chunk(Base):
    __tablename__ = "chunks"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    document_id: Mapped[str] = mapped_column(Text, ForeignKey("documents.id"))
    page: Mapped[int | None] = mapped_column(Integer)
    text: Mapped[str] = mapped_column(Text, nullable=False)
    embedding: Mapped[list[float] | None] = mapped_column(Vector(1536))

    document: Mapped["Document"] = relationship(back_populates="chunks")


class KnowledgeGraph(Base):
    __tablename__ = "knowledge_graphs"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), ForeignKey("users.id"), nullable=False)
    name: Mapped[str] = mapped_column(Text, nullable=False)
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    pipeline_autonomy: Mapped[str] = mapped_column(Text, nullable=False, default="suggest")
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    user: Mapped["User"] = relationship(back_populates="knowledge_graphs")
    sources: Mapped[list["Source"]] = relationship(back_populates="kg")


class Source(Base):
    __tablename__ = "sources"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), ForeignKey("users.id"), nullable=False)
    kg_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), ForeignKey("knowledge_graphs.id"), nullable=False)
    name: Mapped[str] = mapped_column(Text, nullable=False)
    type: Mapped[str] = mapped_column(Text, nullable=False)         # reddit | bluesky | slack | file | url | manual
    sync_mode: Mapped[str] = mapped_column(Text, nullable=False)    # poll | push | one_shot
    sync_state: Mapped[str] = mapped_column(Text, nullable=False, default="pending")  # pending | active | paused | error
    config: Mapped[dict] = mapped_column(JSONB, nullable=False, default=dict)
    auto_promote_threshold: Mapped[float] = mapped_column(Float, nullable=False, default=0.85)
    suggest_threshold: Mapped[float] = mapped_column(Float, nullable=False, default=0.60)
    next_sync_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    last_synced_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    user: Mapped["User"] = relationship(back_populates="sources")
    kg: Mapped["KnowledgeGraph"] = relationship(back_populates="sources")
    snapshots: Mapped[list["Snapshot"]] = relationship(back_populates="source")
    sync_jobs: Mapped[list["SyncJob"]] = relationship(back_populates="source")


class Snapshot(Base):
    __tablename__ = "snapshots"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    source_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("sources.id", ondelete="CASCADE"), nullable=False
    )
    version: Mapped[int] = mapped_column(Integer, nullable=False)
    status: Mapped[str] = mapped_column(Text, nullable=False, default="processing")  # processing | complete | failed
    expected_jobs: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    completed_jobs: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    artifact_count: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    metadata_: Mapped[dict] = mapped_column("metadata", JSONB, nullable=False, default=dict)
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    source: Mapped["Source"] = relationship(back_populates="snapshots")
    artifacts: Mapped[list["Artifact"]] = relationship(back_populates="snapshot")
    sync_jobs: Mapped[list["SyncJob"]] = relationship(back_populates="snapshot")


class SyncJob(Base):
    __tablename__ = "sync_jobs"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    source_id: Mapped[uuid.UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("sources.id", ondelete="CASCADE"), nullable=False
    )
    snapshot_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("snapshots.id", ondelete="CASCADE"), nullable=True
    )
    source_type: Mapped[str] = mapped_column(Text, nullable=False)
    job_type: Mapped[str] = mapped_column(Text, nullable=False)
    payload: Mapped[dict] = mapped_column(JSONB, nullable=False, default=dict)
    priority: Mapped[int] = mapped_column(Integer, nullable=False, default=1)
    scheduled_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )
    attempts: Mapped[int] = mapped_column(Integer, nullable=False, default=0)
    max_attempts: Mapped[int] = mapped_column(Integer, nullable=False, default=3)
    last_error: Mapped[str | None] = mapped_column(Text, nullable=True)
    status: Mapped[str] = mapped_column(Text, nullable=False, default="pending")  # pending | processing | dead

    source: Mapped["Source"] = relationship(back_populates="sync_jobs")
    snapshot: Mapped["Snapshot | None"] = relationship(back_populates="sync_jobs")


class Artifact(Base):
    __tablename__ = "artifacts"

    id: Mapped[str] = mapped_column(Text, primary_key=True)
    source_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("sources.id", ondelete="SET NULL"), nullable=True
    )
    snapshot_id: Mapped[uuid.UUID | None] = mapped_column(
        UUID(as_uuid=True), ForeignKey("snapshots.id", ondelete="SET NULL"), nullable=True
    )
    external_id: Mapped[str | None] = mapped_column(Text, nullable=True)
    version: Mapped[int] = mapped_column(Integer, nullable=False, default=1)
    superseded_by: Mapped[str | None] = mapped_column(
        Text, ForeignKey("artifacts.id", ondelete="SET NULL"), nullable=True
    )
    # type hierarchy:
    #   top-level (grid): 'feed' | 'document' | 'note' | 'webpage'
    #   children:         'post' | 'comment' | 'chunk' | 'message' | 'thread'
    type: Mapped[str] = mapped_column(Text, nullable=False)
    label: Mapped[str] = mapped_column(Text, nullable=False)
    content: Mapped[str] = mapped_column(Text, nullable=False)
    source_url: Mapped[str | None] = mapped_column(Text, nullable=True)
    parent_id: Mapped[str | None] = mapped_column(
        Text, ForeignKey("artifacts.id", ondelete="CASCADE"), nullable=True
    )
    embedding: Mapped[list[float] | None] = mapped_column(Vector(1536), nullable=True)
    metadata_: Mapped[dict] = mapped_column("metadata", JSONB, nullable=False, default=dict)
    enrichment: Mapped[dict | None] = mapped_column(JSONB, nullable=True)
    relevance_score: Mapped[float | None] = mapped_column(Float, nullable=True)
    status: Mapped[str] = mapped_column(Text, nullable=False, default="pending")  # pending | enriched | embedded | in_graph | superseded
    user_status: Mapped[str | None] = mapped_column(Text, nullable=True)  # saved | dismissed | null
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    source: Mapped["Source | None"] = relationship(foreign_keys=[source_id])
    snapshot: Mapped["Snapshot | None"] = relationship(back_populates="artifacts")
    children: Mapped[list["Artifact"]] = relationship(
        "Artifact", foreign_keys="Artifact.parent_id", back_populates="parent"
    )
    parent: Mapped["Artifact | None"] = relationship(
        "Artifact", foreign_keys="Artifact.parent_id", back_populates="children", remote_side="Artifact.id"
    )


class Insight(Base):
    __tablename__ = "insights"

    id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    kg_id: Mapped[uuid.UUID] = mapped_column(UUID(as_uuid=True), ForeignKey("knowledge_graphs.id", ondelete="CASCADE"), nullable=False)
    type: Mapped[str] = mapped_column(Text, nullable=False)  # bridge | convergence | contradiction | trend
    title: Mapped[str] = mapped_column(Text, nullable=False)
    body: Mapped[str] = mapped_column(Text, nullable=False)
    artifact_ids: Mapped[list] = mapped_column(JSONB, nullable=False, default=list)
    entity: Mapped[str | None] = mapped_column(Text, nullable=True)   # for bridge insights
    topic: Mapped[str | None] = mapped_column(Text, nullable=True)    # for convergence insights
    confidence: Mapped[float] = mapped_column(Float, nullable=False, default=0.8)
    user_status: Mapped[str] = mapped_column(Text, nullable=False, default="pending")  # pending | accepted | dismissed
    created_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=lambda: datetime.now(timezone.utc)
    )

    kg: Mapped["KnowledgeGraph"] = relationship(foreign_keys=[kg_id])

"""add sources, snapshots, sync_jobs; update artifacts

Revision ID: d4e5f6a7b8c9
Revises: c3d4e5f6a7b8
Create Date: 2026-03-26 00:00:00.000000

"""
from typing import Sequence, Union
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects.postgresql import JSONB, UUID
import pgvector.sqlalchemy

revision: str = 'd4e5f6a7b8c9'
down_revision: Union[str, Sequence[str], None] = 'c3d4e5f6a7b8'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # ── knowledge_graphs ────────────────────────────────────────────────────
    op.create_table(
        'knowledge_graphs',
        sa.Column('id', UUID(), nullable=False),
        sa.Column('user_id', UUID(), sa.ForeignKey('users.id'), nullable=False),
        sa.Column('name', sa.Text(), nullable=False),
        sa.Column('description', sa.Text(), nullable=True),
        sa.Column('pipeline_autonomy', sa.Text(), nullable=False, server_default='suggest'),
        sa.Column('created_at', sa.DateTime(timezone=True), nullable=False,
                  server_default=sa.func.now()),
        sa.PrimaryKeyConstraint('id'),
    )

    # ── sources ─────────────────────────────────────────────────────────────
    op.create_table(
        'sources',
        sa.Column('id', UUID(), nullable=False),
        sa.Column('user_id', UUID(), sa.ForeignKey('users.id'), nullable=False),
        sa.Column('kg_id', UUID(), sa.ForeignKey('knowledge_graphs.id'), nullable=False),
        sa.Column('name', sa.Text(), nullable=False),
        sa.Column('type', sa.Text(), nullable=False),
        sa.Column('sync_mode', sa.Text(), nullable=False),
        sa.Column('sync_state', sa.Text(), nullable=False, server_default='pending'),
        sa.Column('config', JSONB(), nullable=False, server_default='{}'),
        sa.Column('auto_promote_threshold', sa.Float(), nullable=False, server_default='0.85'),
        sa.Column('suggest_threshold', sa.Float(), nullable=False, server_default='0.60'),
        sa.Column('next_sync_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('last_synced_at', sa.DateTime(timezone=True), nullable=True),
        sa.Column('created_at', sa.DateTime(timezone=True), nullable=False,
                  server_default=sa.func.now()),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index('idx_sources_due', 'sources', ['next_sync_at'],
                    postgresql_where=sa.text("sync_mode = 'poll' AND sync_state = 'active'"))

    # ── snapshots ────────────────────────────────────────────────────────────
    op.create_table(
        'snapshots',
        sa.Column('id', UUID(), nullable=False),
        sa.Column('source_id', UUID(), sa.ForeignKey('sources.id', ondelete='CASCADE'),
                  nullable=False),
        sa.Column('version', sa.Integer(), nullable=False),
        sa.Column('status', sa.Text(), nullable=False, server_default='processing'),
        sa.Column('expected_jobs', sa.Integer(), nullable=False, server_default='0'),
        sa.Column('completed_jobs', sa.Integer(), nullable=False, server_default='0'),
        sa.Column('artifact_count', sa.Integer(), nullable=False, server_default='0'),
        sa.Column('metadata', JSONB(), nullable=False, server_default='{}'),
        sa.Column('created_at', sa.DateTime(timezone=True), nullable=False,
                  server_default=sa.func.now()),
        sa.PrimaryKeyConstraint('id'),
        sa.UniqueConstraint('source_id', 'version', name='uq_snapshot_source_version'),
    )
    op.create_index('idx_snapshots_source', 'snapshots', ['source_id'])

    # ── sync_jobs ────────────────────────────────────────────────────────────
    op.create_table(
        'sync_jobs',
        sa.Column('id', UUID(), nullable=False),
        sa.Column('source_id', UUID(), sa.ForeignKey('sources.id', ondelete='CASCADE'),
                  nullable=False),
        sa.Column('snapshot_id', UUID(), sa.ForeignKey('snapshots.id', ondelete='CASCADE'),
                  nullable=True),
        sa.Column('source_type', sa.Text(), nullable=False),
        sa.Column('job_type', sa.Text(), nullable=False),
        sa.Column('payload', JSONB(), nullable=False, server_default='{}'),
        sa.Column('priority', sa.Integer(), nullable=False, server_default='1'),
        sa.Column('scheduled_at', sa.DateTime(timezone=True), nullable=False,
                  server_default=sa.func.now()),
        sa.Column('created_at', sa.DateTime(timezone=True), nullable=False,
                  server_default=sa.func.now()),
        sa.Column('attempts', sa.Integer(), nullable=False, server_default='0'),
        sa.Column('max_attempts', sa.Integer(), nullable=False, server_default='3'),
        sa.Column('last_error', sa.Text(), nullable=True),
        sa.Column('status', sa.Text(), nullable=False, server_default='pending'),
        sa.PrimaryKeyConstraint('id'),
    )
    op.create_index(
        'idx_sync_jobs_dequeue',
        'sync_jobs',
        ['priority', 'scheduled_at', 'created_at'],
        postgresql_where=sa.text("status = 'pending'"),
    )

    # ── update artifacts ─────────────────────────────────────────────────────
    op.add_column('artifacts',
        sa.Column('source_id', UUID(), sa.ForeignKey('sources.id', ondelete='SET NULL'),
                  nullable=True))
    op.add_column('artifacts',
        sa.Column('snapshot_id', UUID(), sa.ForeignKey('snapshots.id', ondelete='SET NULL'),
                  nullable=True))
    op.add_column('artifacts',
        sa.Column('external_id', sa.Text(), nullable=True))
    op.add_column('artifacts',
        sa.Column('version', sa.Integer(), nullable=False, server_default='1'))
    op.add_column('artifacts',
        sa.Column('superseded_by', sa.Text(),
                  sa.ForeignKey('artifacts.id', ondelete='SET NULL'), nullable=True))
    op.add_column('artifacts',
        sa.Column('embedding', pgvector.sqlalchemy.Vector(1536), nullable=True))
    op.add_column('artifacts',
        sa.Column('metadata', JSONB(), nullable=False, server_default='{}'))
    op.add_column('artifacts',
        sa.Column('relevance_score', sa.Float(), nullable=True))
    op.add_column('artifacts',
        sa.Column('status', sa.Text(), nullable=False, server_default='pending'))

    op.create_index('idx_artifacts_source', 'artifacts', ['source_id'])
    op.create_index('idx_artifacts_snapshot', 'artifacts', ['snapshot_id'])
    op.create_index(
        'idx_artifacts_external',
        'artifacts',
        ['source_id', 'external_id'],
        postgresql_where=sa.text("external_id IS NOT NULL"),
    )
    op.create_index(
        'idx_artifacts_pending',
        'artifacts',
        ['created_at'],
        postgresql_where=sa.text("status = 'pending'"),
    )


def downgrade() -> None:
    # artifacts
    op.drop_index('idx_artifacts_pending')
    op.drop_index('idx_artifacts_external')
    op.drop_index('idx_artifacts_snapshot')
    op.drop_index('idx_artifacts_source')
    op.drop_column('artifacts', 'status')
    op.drop_column('artifacts', 'relevance_score')
    op.drop_column('artifacts', 'metadata')
    op.drop_column('artifacts', 'embedding')
    op.drop_column('artifacts', 'superseded_by')
    op.drop_column('artifacts', 'version')
    op.drop_column('artifacts', 'external_id')
    op.drop_column('artifacts', 'snapshot_id')
    op.drop_column('artifacts', 'source_id')

    # sync_jobs
    op.drop_index('idx_sync_jobs_dequeue')
    op.drop_table('sync_jobs')

    # snapshots
    op.drop_index('idx_snapshots_source')
    op.drop_table('snapshots')

    # sources
    op.drop_index('idx_sources_due')
    op.drop_table('sources')

    # knowledge_graphs
    op.drop_table('knowledge_graphs')

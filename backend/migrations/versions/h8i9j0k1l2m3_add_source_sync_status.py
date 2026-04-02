"""add source sync_status

Revision ID: h8i9j0k1l2m3
Revises: g7h8i9j0k1l2
Create Date: 2026-03-31

Replaces next_sync_at scheduling with availability-based scheduling.
sync_status tracks idle | queued | syncing to prevent double-claiming.
"""
from alembic import op
import sqlalchemy as sa

revision = 'h8i9j0k1l2m3'
down_revision = 'g7h8i9j0k1l2'
branch_labels = None
depends_on = None


def upgrade():
    op.add_column('sources',
        sa.Column('sync_status', sa.Text(), nullable=False, server_default='idle')
    )
    op.create_index(
        'idx_sources_claimable',
        'sources',
        ['last_synced_at'],
        postgresql_where=sa.text(
            "sync_mode = 'poll' AND sync_state = 'active' AND sync_status = 'idle'"
        )
    )


def downgrade():
    op.drop_index('idx_sources_claimable', table_name='sources')
    op.drop_column('sources', 'sync_status')

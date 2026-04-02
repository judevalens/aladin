"""add source last_sync_started_at

Revision ID: i9j0k1l2m3n4
Revises: h8i9j0k1l2m3
Create Date: 2026-03-31

Tracks when a sync job was picked up by a worker, separate from last_synced_at
which only stamps on completion. Useful for detecting stuck syncs and measuring
sync duration.
"""
from alembic import op
import sqlalchemy as sa

revision = 'i9j0k1l2m3n4'
down_revision = 'h8i9j0k1l2m3'
branch_labels = None
depends_on = None


def upgrade():
    op.add_column('sources',
        sa.Column('last_sync_started_at', sa.DateTime(timezone=True), nullable=True)
    )


def downgrade():
    op.drop_column('sources', 'last_sync_started_at')

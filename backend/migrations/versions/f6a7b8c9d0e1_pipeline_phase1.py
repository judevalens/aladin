"""pipeline phase1: user_status on artifacts + insights table

Revision ID: f6a7b8c9d0e1
Revises: e5f6a7b8c9d0
Create Date: 2026-03-26 00:00:00.000000
"""
from typing import Sequence, Union
from alembic import op
import sqlalchemy as sa
from sqlalchemy.dialects.postgresql import JSONB, UUID

revision: str = 'f6a7b8c9d0e1'
down_revision: Union[str, Sequence[str], None] = 'e5f6a7b8c9d0'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Add user_status to artifacts (save / dismiss)
    op.add_column('artifacts',
        sa.Column('user_status', sa.Text(), nullable=True))

    # Insights table
    op.create_table(
        'insights',
        sa.Column('id', UUID(as_uuid=True), primary_key=True, server_default=sa.text('gen_random_uuid()')),
        sa.Column('kg_id', UUID(as_uuid=True), sa.ForeignKey('knowledge_graphs.id', ondelete='CASCADE'), nullable=False),
        sa.Column('type', sa.Text(), nullable=False),
        sa.Column('title', sa.Text(), nullable=False),
        sa.Column('body', sa.Text(), nullable=False),
        sa.Column('artifact_ids', JSONB, nullable=False, server_default='[]'),
        sa.Column('entity', sa.Text(), nullable=True),
        sa.Column('topic', sa.Text(), nullable=True),
        sa.Column('confidence', sa.Float(), nullable=False, server_default='0.8'),
        sa.Column('user_status', sa.Text(), nullable=False, server_default='pending'),
        sa.Column('created_at', sa.DateTime(timezone=True), server_default=sa.text('now()')),
    )

    op.create_index('idx_insights_kg', 'insights', ['kg_id'])
    op.create_index('idx_insights_user_status', 'insights', ['user_status'])
    op.create_index('idx_artifacts_user_status', 'artifacts', ['user_status'],
                    postgresql_where=sa.text('user_status IS NOT NULL'))

    # Index for pipeline worker queries
    op.create_index('idx_artifacts_pipeline_status', 'artifacts', ['status', 'created_at'],
                    postgresql_where=sa.text("status IN ('pending', 'enriched', 'embedded')"))


def downgrade() -> None:
    op.drop_index('idx_artifacts_pipeline_status')
    op.drop_index('idx_artifacts_user_status')
    op.drop_index('idx_insights_user_status')
    op.drop_index('idx_insights_kg')
    op.drop_table('insights')
    op.drop_column('artifacts', 'user_status')

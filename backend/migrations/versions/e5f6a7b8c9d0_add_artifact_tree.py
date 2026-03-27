"""add artifact tree (parent_id, updated types)

Revision ID: e5f6a7b8c9d0
Revises: d4e5f6a7b8c9
Create Date: 2026-03-27 00:00:00.000000

"""
from typing import Sequence, Union
from alembic import op
import sqlalchemy as sa

revision: str = 'e5f6a7b8c9d0'
down_revision: Union[str, Sequence[str], None] = 'd4e5f6a7b8c9'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    op.add_column('artifacts',
        sa.Column('parent_id', sa.Text(),
                  sa.ForeignKey('artifacts.id', ondelete='CASCADE'),
                  nullable=True))

    op.create_index('idx_artifacts_parent', 'artifacts', ['parent_id'],
                    postgresql_where=sa.text('parent_id IS NOT NULL'))

    # Top-level artifacts only (grid view)
    op.create_index('idx_artifacts_top_level', 'artifacts', ['source_id', 'created_at'],
                    postgresql_where=sa.text('parent_id IS NULL'))


def downgrade() -> None:
    op.drop_index('idx_artifacts_top_level')
    op.drop_index('idx_artifacts_parent')
    op.drop_column('artifacts', 'parent_id')

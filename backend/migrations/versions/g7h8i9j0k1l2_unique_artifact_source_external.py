"""unique constraint on artifacts(source_id, external_id)

Revision ID: g7h8i9j0k1l2
Revises: f6a7b8c9d0e1
Create Date: 2026-03-27 00:00:00.000000
"""
from typing import Sequence, Union
import sqlalchemy as sa
from alembic import op

revision: str = 'g7h8i9j0k1l2'
down_revision: Union[str, Sequence[str], None] = 'f6a7b8c9d0e1'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    # Required for ON CONFLICT (source_id, external_id) DO NOTHING in SaveComplete
    op.create_index(
        'uq_artifacts_source_external',
        'artifacts',
        ['source_id', 'external_id'],
        unique=True,
        postgresql_where=sa.text("external_id IS NOT NULL"),
    )


def downgrade() -> None:
    op.drop_index('uq_artifacts_source_external', table_name='artifacts')

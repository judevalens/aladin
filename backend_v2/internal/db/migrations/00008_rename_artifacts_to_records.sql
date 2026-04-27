-- +goose Up
DO $$
BEGIN
    IF to_regclass('public.artifacts') IS NOT NULL AND to_regclass('public.records') IS NULL THEN
        ALTER TABLE artifacts RENAME TO records;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'snapshots' AND column_name = 'artifact_count'
    ) THEN
        ALTER TABLE snapshots RENAME COLUMN artifact_count TO record_count;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'insights' AND column_name = 'artifact_ids'
    ) THEN
        ALTER TABLE insights RENAME COLUMN artifact_ids TO record_ids;
    END IF;

    IF to_regclass('public.idx_artifacts_source') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_source RENAME TO idx_records_source;
    END IF;
    IF to_regclass('public.idx_artifacts_snapshot') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_snapshot RENAME TO idx_records_snapshot;
    END IF;
    IF to_regclass('public.idx_artifacts_external') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_external RENAME TO idx_records_external;
    END IF;
    IF to_regclass('public.idx_artifacts_pending') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_pending RENAME TO idx_records_pending;
    END IF;
    IF to_regclass('public.idx_artifacts_parent') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_parent RENAME TO idx_records_parent;
    END IF;
    IF to_regclass('public.idx_artifacts_top_level') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_top_level RENAME TO idx_records_top_level;
    END IF;
    IF to_regclass('public.idx_artifacts_user_status') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_user_status RENAME TO idx_records_user_status;
    END IF;
    IF to_regclass('public.idx_artifacts_pipeline_status') IS NOT NULL THEN
        ALTER INDEX idx_artifacts_pipeline_status RENAME TO idx_records_pipeline_status;
    END IF;
    IF to_regclass('public.uq_artifacts_source_external') IS NOT NULL THEN
        ALTER INDEX uq_artifacts_source_external RENAME TO uq_records_source_external;
    END IF;
END $$;

-- +goose Down
DO $$
BEGIN
    IF to_regclass('public.records') IS NOT NULL AND to_regclass('public.artifacts') IS NULL THEN
        ALTER TABLE records RENAME TO artifacts;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'snapshots' AND column_name = 'record_count'
    ) THEN
        ALTER TABLE snapshots RENAME COLUMN record_count TO artifact_count;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'insights' AND column_name = 'record_ids'
    ) THEN
        ALTER TABLE insights RENAME COLUMN record_ids TO artifact_ids;
    END IF;

    IF to_regclass('public.idx_records_source') IS NOT NULL THEN
        ALTER INDEX idx_records_source RENAME TO idx_artifacts_source;
    END IF;
    IF to_regclass('public.idx_records_snapshot') IS NOT NULL THEN
        ALTER INDEX idx_records_snapshot RENAME TO idx_artifacts_snapshot;
    END IF;
    IF to_regclass('public.idx_records_external') IS NOT NULL THEN
        ALTER INDEX idx_records_external RENAME TO idx_artifacts_external;
    END IF;
    IF to_regclass('public.idx_records_pending') IS NOT NULL THEN
        ALTER INDEX idx_records_pending RENAME TO idx_artifacts_pending;
    END IF;
    IF to_regclass('public.idx_records_parent') IS NOT NULL THEN
        ALTER INDEX idx_records_parent RENAME TO idx_artifacts_parent;
    END IF;
    IF to_regclass('public.idx_records_top_level') IS NOT NULL THEN
        ALTER INDEX idx_records_top_level RENAME TO idx_artifacts_top_level;
    END IF;
    IF to_regclass('public.idx_records_user_status') IS NOT NULL THEN
        ALTER INDEX idx_records_user_status RENAME TO idx_artifacts_user_status;
    END IF;
    IF to_regclass('public.idx_records_pipeline_status') IS NOT NULL THEN
        ALTER INDEX idx_records_pipeline_status RENAME TO idx_artifacts_pipeline_status;
    END IF;
    IF to_regclass('public.uq_records_source_external') IS NOT NULL THEN
        ALTER INDEX uq_records_source_external RENAME TO uq_artifacts_source_external;
    END IF;
END $$;

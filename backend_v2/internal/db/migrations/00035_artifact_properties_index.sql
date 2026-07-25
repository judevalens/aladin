-- +goose Up
-- H1c — make typed artifact properties QUERYABLE (the frontmatter payoff).
--
-- Properties live at artifacts.metadata->'properties' as a JSON ARRAY of
--   {key, type, value, values?, options?}
-- so a lookup is jsonb containment:
--   metadata @> '{"properties":[{"key":"Status","value":"Live"}]}'
-- (array containment matches when every element of the probe is contained in some element of the
-- stored array, so a one-element probe matches on key+value regardless of position or the other
-- fields like `type`.)
--
-- jsonb_path_ops rather than the default jsonb_ops: it indexes only containment (@>), which is
-- exactly this query, and produces a markedly smaller/faster index. The trade-off — no support for
-- key-existence (?) operators — is fine here; facet reads scan a different path.
CREATE INDEX artifacts_metadata_gin ON public.artifacts USING gin (metadata jsonb_path_ops);

-- +goose Down
DROP INDEX IF EXISTS public.artifacts_metadata_gin;

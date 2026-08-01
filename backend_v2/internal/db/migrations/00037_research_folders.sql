-- +goose Up
-- The research bench, §5 of design/RESEARCH_SURFACE_PRD.md: the unit of work is a
-- RESEARCH FOLDER — a tree node with kind='research' plus a 1:1 extension row holding
-- strategy facts. Same shape the repo already uses twice (watchlists: one entity + a kind
-- + a definition payload; the trading entity model: thin entities + kind + hard 1:1
-- extension tables). One tree, one set of components, a discriminator.
--
-- Why a kind and not a plain folder (§5): structural slots that exist at creation, a
-- constrained `+` menu, seeded typed properties, a stable address for agents, and — the
-- clearest tell — *state*. Folders don't have state.
--
-- The node rides the EXISTING tree sync spine: sync frames key entityKind off
-- tree_nodes.kind, so a research node is just a third kind on the same producer,
-- snapshot, and client store. No new synced entity kind.

-- tree_nodes gains a third kind. Research nodes carry the FOLDER shape (a container: no
-- artifact_id, a title of their own), so the shape check gets a research branch too.
ALTER TABLE public.tree_nodes DROP CONSTRAINT IF EXISTS tree_nodes_kind_check;
ALTER TABLE public.tree_nodes
    ADD CONSTRAINT tree_nodes_kind_check
    CHECK (kind = ANY (ARRAY['folder'::text, 'artifact'::text, 'research'::text]));

ALTER TABLE public.tree_nodes DROP CONSTRAINT IF EXISTS tree_nodes_check;
ALTER TABLE public.tree_nodes
    ADD CONSTRAINT tree_nodes_check CHECK (
        (kind IN ('folder', 'research') AND artifact_id IS NULL AND title IS NOT NULL)
     OR (kind = 'artifact' AND artifact_id IS NOT NULL)
    );

-- The 1:1 extension. Deliberately SPARSE: §5 says one research folder = one strategy
-- "including the case where no code exists yet". Every column below the identity is
-- nullable or defaulted, so the row exists from creation with nothing filled in — that is
-- what makes the structural slots present-but-empty rather than absent.
CREATE TABLE public.research_strategies (
    node_id     text        PRIMARY KEY REFERENCES public.tree_nodes (id) ON DELETE CASCADE,
    user_id     uuid        NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,

    -- §2 of TRADING_PRD puts `hypothesis` as a column on the strategy, not an object above
    -- it. This is the prose region of the Overview (§15).
    hypothesis  text,

    -- §7: a strategy arrives from a git import, a local directory, or authored in Aladin,
    -- and all three converge on one representation (a directory + manifest, content-hashed).
    source_kind text        NOT NULL DEFAULT 'authored',
    source_ref  text,                                   -- git remote or local path
    commit_sha  text,                                   -- §7: pin a SHA, NEVER a branch

    -- §7: the strategy declares its parameter schema; the manifest supplies the values.
    -- Opaque here — the engine owns its shape, Aladin introspects it to render forms.
    manifest    jsonb       NOT NULL DEFAULT '{}'::jsonb,

    -- §9: every run pins a code hash, and runs are immutable. Null until a first run.
    code_hash   text,

    -- §17's universe field. Opaque jsonb ON PURPOSE: D6 (as-of universe resolution) is
    -- still open in TRADING_PRD §6 and it decides this shape. Don't harden it early.
    universe    jsonb       NOT NULL DEFAULT '{}'::jsonb,

    -- §8: event-driven is the primitive; batch is an explicit, VISIBLY MARKED opt-out.
    exec_mode   text        NOT NULL DEFAULT 'event',

    -- §5: the folder has state — this is the clearest tell it isn't a plain folder.
    -- Only idle/offline are reachable until the engine lands (§22).
    run_state   text        NOT NULL DEFAULT 'idle',

    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT research_source_kind_chk CHECK (source_kind IN ('git', 'local', 'authored')),
    CONSTRAINT research_exec_mode_chk   CHECK (exec_mode IN ('event', 'batch')),
    CONSTRAINT research_run_state_chk   CHECK (run_state IN ('idle', 'running', 'armed', 'offline')),
    -- §7: a git import without a pinned SHA is the bug this constraint exists to prevent.
    CONSTRAINT research_git_needs_sha_chk CHECK (source_kind <> 'git' OR commit_sha IS NOT NULL)
);

CREATE INDEX research_strategies_user_idx ON public.research_strategies (user_id);

-- +goose Down
DROP TABLE IF EXISTS public.research_strategies;

ALTER TABLE public.tree_nodes DROP CONSTRAINT IF EXISTS tree_nodes_check;
ALTER TABLE public.tree_nodes
    ADD CONSTRAINT tree_nodes_check CHECK (
        ((kind = 'folder'::text) AND (artifact_id IS NULL) AND (title IS NOT NULL))
     OR ((kind = 'artifact'::text) AND (artifact_id IS NOT NULL))
    );

ALTER TABLE public.tree_nodes DROP CONSTRAINT IF EXISTS tree_nodes_kind_check;
ALTER TABLE public.tree_nodes
    ADD CONSTRAINT tree_nodes_kind_check
    CHECK (kind = ANY (ARRAY['folder'::text, 'artifact'::text]));

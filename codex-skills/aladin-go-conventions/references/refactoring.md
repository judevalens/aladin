# Refactoring toward Aladin conventions

This reference is loaded when the skill is in refactor mode, meaning the user asked for an audit, plan, or fix of existing code rather than placement of new code.

The conventions themselves live in `SKILL.md`. This file explains how to compare current code against those conventions and move it toward them safely.

## The four phases

Walk every refactor request through these phases, in order:

1. Scope: confirm what the user wants audited.
2. Audit: compare current code against the conventions and list violations.
3. Plan: turn the violations into an ordered execution plan and get approval.
4. Execute: work the plan step by step, keeping the build mostly green.

Do not skip from finding violations straight to editing code.

## Phase 1: Scope

Treat the requested file, package, or domain as the exact scope. Do not silently broaden it.

If scope is ambiguous, confirm it first.

Good examples:

- "I'll audit `internal/ingest` as the scope. Confirm?"
- "Just `internal/ingest/api/reddit/refresh.go`, right?"

Bad example:

- Auditing the whole repo when the user mentioned one file

If you notice issues outside scope, mention them separately at the end of the audit and keep them out of the active plan.

## Phase 2: Audit

Use this checklist against the scoped code.

### Structural checks

1. Is the code in `internal/` rather than `pkg/`?
2. Is the top-level package domain-named rather than a graveyard like `utils` or `common`?
3. Inside the domain, is code split into `service/`, `repo/`, and `api/`, with subdomain packages under each?

### API checks

4. Does the `api/<subdomain>` package import `repo` directly?
5. Does it contain domain-state branching, orchestration, or transaction logic?
6. Does it call services for everything except parsing, validation, and response shaping?

### Service checks

7. Does the service accept interfaces for repo dependencies rather than concrete types?
8. Are those interfaces defined in the service package?
9. Does the service import another domain's `repo` package directly?
10. Does the service contain raw storage or query code that belongs in `repo`?

### Repo checks

11. Does the repo contain business-logic conditionals?
12. Does the repo call into a service?
13. Does the repo import another domain's packages?
14. Would the repo boundary survive a swap from the current store to an in-memory implementation?

### Type placement checks

15. Are there types in `internal/domain` used by only one domain?
16. Are there duplicated cross-domain types that should move into `internal/domain`?
17. Does `internal/domain` import other `internal/*` packages or non-stdlib third-party packages beyond narrow type-level exceptions like a UUID helper?

### Wiring checks

18. Does `cmd/<binary>/main.go` contain business logic?
19. Are dependencies built bottom-up and injected through constructors?

### Import-graph sanity check

Healthy shape:

- `api/<subdomain>` imports `service/<subdomain>`
- `service` imports `domain`, stdlib, and any consumer-declared cross-domain interfaces
- `repo` imports `domain`, storage clients, and stdlib
- `domain` imports stdlib only

Quick checks:

```bash
grep -r "aladin/internal/<domain>/repo" internal/<domain>/api/
grep -r "aladin/internal/<domain>/repo" internal/<domain>/service/
```

Both should usually be empty.

## Phase 3: Plan

Before editing anything, show the user a plan in this exact shape:

```markdown
# Refactor plan: <scope>

## Summary
<1-3 sentences: current problem, target end state.>

## Violations found
<Numbered list. Each entry: file:line range, rule violated, one-line description.>

## Proposed steps
<Ordered list. Each step: action, files affected, build state expected after, rationale.>

## Out of scope (noted but not addressed)
<Findings outside the requested scope.>
```

### Step ordering

Prioritize using all three axes together:

- severity tier
- risk
- dependency order

Typical order:

1. Move or rename files.
2. Extract consumer-defined interfaces in `service`.
3. Move data-access code into `repo`.
4. Move business logic out of `api/<subdomain>` into `service/<subdomain>`.
5. Update `cmd/` wiring last.

### Build state labels

For each step, predict one of:

- `green`: compiles and tests pass
- `yellow`: compiles, some tests broken
- `red`: does not compile

`Red` is allowed only briefly and only if the immediately following step restores green. Do not end a session in red.

## Phase 4: Execute

After the user approves the plan:

1. State which step you are about to do.
2. Make the change.
3. Run `go build ./...` and, when relevant, `go test ./...`.
4. Show what changed.
5. Wait for approval before continuing to the next step.

If a step behaves differently from the plan, stop and re-plan. Do not paper over it.

## When the refactor grows

If a new issue blocks the current plan, stop and ask whether to expand scope or descale the step.

If a new issue is independent, note it as out of scope and finish the current refactor first.

Do not quietly absorb extra work into the plan.

## Avoid these mistakes

- Do not refactor and add features in the same pass.
- Do not restyle unrelated code.
- Do not move types into `internal/domain` speculatively.
- Do not add abstractions beyond what the conventions require.
- Do not delete code without explicit confirmation, even if it looks dead.

## Example: small refactor walkthrough

Use this example as the model for narrow refactors where an API package feels too thick.

User: "Refactor `internal/ingest/api/reddit/refresh.go` - it feels too thick."

Phase 1, scope:

- Confirm the scope is just `internal/ingest/api/reddit/refresh.go` and whatever minimal dependencies the change needs.

Phase 2, audit:

1. API package imports `repo/reddit` and calls Redis directly.
2. API package contains cursor-freshness business logic.
3. API package builds the async payload inline.

Phase 3, plan:

```markdown
# Refactor plan: internal/ingest/api/reddit/refresh.go

## Summary
API package is doing repo access and business logic that belong in the service.
Goal: the API package parses the request, calls the service, and translates the response. The service owns the cursor freshness rule and the enqueue.

## Violations found
1. api/reddit/refresh.go:42-58 - direct Redis call, violates API thinness
2. api/reddit/refresh.go:60-74 - cursor freshness conditional, business logic in API package
3. api/reddit/refresh.go:80-95 - async payload construction in API package

## Proposed steps
1. Add `RefreshCursor(ctx, sub) error` to `service/reddit.Service`. Move freshness logic, Redis access, and enqueue into it. Build: green. Rationale: additive boundary first inside the subdomain.
2. Replace the API entrypoint body with parse + call `service/reddit.Service.RefreshCursor` + write response. Build: green. Rationale: thin API package once the service API exists.
3. Remove the now-unused repo import from the API package. Build: green. Rationale: cleanup after behavior is moved.

## Out of scope (noted but not addressed)
- `api/twitter` appears to have similar problems.
```

Phase 4, execute:

- Wait for approval.
- Do step 1, run build or tests, show the change, and wait again.
- Continue one approved step at a time.

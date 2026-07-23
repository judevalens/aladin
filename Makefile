.PHONY: help backend mcp blocknote blocknote-test copilot-agent copilot-agent-test check-blocknote-versions tokens check-tokens nuke-local-db nuke-clients tauri-client-b db-up db-down test-db-up test-db-down test-go nango-up nango-down nango-logs env-nango ngrok-ensure worker-go api-go ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles ops-backfill-instruments ops-backfill-bars

# --- Isolated sandbox stack (docker-compose.test.yml) -----------------------
# A throwaway mirror of the dev infra on DISTINCT ports, namespaced under the
# `aladin-test` project, used for agent/automated testing so the real `aladin-*`
# dev stack and its data are never touched.
TEST_COMPOSE := docker compose -f docker-compose.test.yml -p aladin-test
TEST_DATABASE_URL := postgres://aladin:password@localhost:5444/aladin

# PROD stack (isolated third stack; see PROD.md). Compose reads secrets from
# backend_v2/.env.prod via --env-file. PROD_PROFILES selects which app processes
# start (override to run lean, e.g. PROD_PROFILES=api,collab for notes-only).
PROD_COMPOSE := docker compose -p aladin-prod --env-file backend_v2/.env.prod -f docker-compose.prod.yml
PROD_PROFILES ?= api,worker,mcp,collab,copilot
# Env keys the copilot-agent Node sidecar needs from backend_v2/.env (it does
# not load .env itself, unlike the Go binaries' godotenv).
COPILOT_AGENT_ENV_KEYS = --key ANTHROPIC_API_KEY --key COPILOT_MODEL --key COPILOT_AGENT_SHARED_SECRET --key ALADIN_MCP_URL --key COPILOT_AUTH

# Doc Surface page file store (users/{userId}/pages/{pageId}/...). The API
# process SERVES built dist from here; the MCP process WRITES files + BUILDS into
# it. Both MUST resolve to the same path — set it once here. Override with
# `make backend DATA_VOLUME_PATH=/abs/path`.
DATA_VOLUME_PATH ?= $(CURDIR)/backend_v2/data

help: ## List available make targets
	@awk 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

backend: ngrok-ensure ## Run the blocknote + copilot-agent sidecars (local) + the Go API on :8000 (needs infra up; see `make db-up`)
	@(cd services/blocknote && exec node server.js) & \
	SIDECAR_PID=$$!; \
	(eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env $(COPILOT_AGENT_ENV_KEYS))" && cd services/copilot-agent && exec node server.js) & \
	AGENT_PID=$$!; \
	trap 'kill $$SIDECAR_PID $$AGENT_PID 2>/dev/null' EXIT INT TERM; \
	echo ">> blocknote sidecar pid $$SIDECAR_PID (converter :3500, collab :3501)"; \
	echo ">> copilot-agent sidecar pid $$AGENT_PID (:3550)"; \
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && API_ADDR=:8000 DATA_VOLUME_PATH=$(DATA_VOLUME_PATH) go run ./cmd/api

mcp: ## Run the MCP page server on port 8090
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && MCP_HTTP_ADDR=:8090 DATA_VOLUME_PATH=$(DATA_VOLUME_PATH) go run ./cmd/mcp

blocknote: ## Run the blocknote Node sidecar locally (converter :3500 + collab :3501); needs postgres + Go API up
	cd services/blocknote && npm start

blocknote-test: ## Run the blocknote Node service unit tests
	cd services/blocknote && npm test

copilot-agent: ## Run the copilot-agent sidecar locally (:3550); needs the MCP server (make mcp) + ANTHROPIC_API_KEY
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env $(COPILOT_AGENT_ENV_KEYS))" && cd services/copilot-agent && npm start

copilot-agent-test: ## Run the copilot-agent unit tests
	cd services/copilot-agent && npm test

check-blocknote-versions: ## Fail if @blocknote/* + yjs versions drift between aladin_react and services/blocknote
	bash scripts/check-blocknote-versions.sh

tokens: ## Regenerate the backend theme copy from the canonical aladin_react/src/theme.css (single source of truth)
	cp aladin_react/src/theme.css backend_v2/internal/docsurface/theme.css
	@echo ">> backend theme.css regenerated from aladin_react/src/theme.css"

check-tokens: ## Fail if the backend theme copy drifts from the canonical (run `make tokens` to fix)
	@diff -u aladin_react/src/theme.css backend_v2/internal/docsurface/theme.css \
		&& echo ">> tokens in sync" \
		|| (echo ">> ERROR: backend theme.css is stale — run 'make tokens'"; exit 1)

nuke-local-db: ## Wipe the Tauri local SQLite when a schema change breaks the local cache (close the app first; FORCE=1 to override)
	bash scripts/nuke-local-db.sh

nuke-clients: ## Wipe the local SQLite of ALL Tauri clients (com.aladin.react*) — use after a server DB reset; close all apps first, FORCE=1 to override
	bash scripts/nuke-client-dbs.sh

tauri-client-b: ## Build + launch a 2nd Tauri client (own identity/login) to simulate a second collab user (macOS)
	@echo ">> building a second Tauri client (id com.aladin.react.b); first build takes a few minutes..."
	cd aladin_react && npm run tauri:build -- --debug --bundles app --config '{"identifier":"com.aladin.react.b","productName":"AladinB"}'
	open "aladin_react/src-tauri/target/debug/bundle/macos/AladinB.app"

db-up: ## Start local Docker infrastructure
	docker compose up -d

db-down: ## Stop local Docker infrastructure
	docker compose down

test-db-up: ## Start the ISOLATED sandbox stack (pg :5444, neo4j :7475/:7688, redis :6380); coexists with the dev stack
	$(TEST_COMPOSE) up -d postgres neo4j redis

test-db-down: ## Stop the sandbox stack (pass ARGS=-v to also drop sandbox volumes)
	$(TEST_COMPOSE) down $(ARGS)

test-go: ## Run Go tests against the SANDBOX db (TEST_DATABASE_URL -> :5444); boots sandbox postgres first
	$(TEST_COMPOSE) up -d postgres
	@echo ">> waiting for sandbox postgres on :5444..."
	@until docker exec aladin-test-postgres pg_isready -U aladin -d aladin >/dev/null 2>&1; do sleep 1; done
	cd backend_v2 && env -u DATABASE_URL TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./...

nango-up: ## Start local Nango self-hosted services
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && docker compose -f docker-compose.nango.yml up -d

nango-down: ## Stop local Nango self-hosted services
	docker compose -f docker-compose.nango.yml down

nango-logs: ## Tail local Nango logs
	docker compose -f docker-compose.nango.yml logs -f

env-nango: ## Print Nango env exports from backend_v2/.env
	@python3 scripts/ops/read_env_keys.py --env backend_v2/.env

ngrok-ensure: ## Ensure an ngrok tunnel for local Nango webhooks
	@python3 scripts/ops/ngrok_ensure.py --env backend_v2/.env --port 8000

worker-go: ngrok-ensure ## Run the Go worker; optional CONCURRENCY=24
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && WORKER_CONCURRENCY=$(or $(CONCURRENCY),16) go run ./cmd/worker

api-go: ## Run the Go backend API
	cd backend_v2 && DATA_VOLUME_PATH=$(DATA_VOLUME_PATH) go run ./cmd/api

ops-backfill-entities: ## Resolve entities for all enriched records into the entity layer
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-entities

ops-backfill-graph: ## Project the entity + claim layer into Neo4j (the connection lens). Needs NEO4J_URI.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-graph

ops-backfill-instruments: ## Pull the Alpaca Assets universe into the instruments registry (T1). Needs ALPACA_API_KEY/SECRET.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-instruments

ops-backfill-bars: ## Pull historical daily bars from Alpaca into the bars store. Needs ALPACA_API_KEY/SECRET.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-bars

ops-backfill-signals: ## Re-emit durable signal frames for all existing claims to their subscribers (Signals sync backfill).
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-signals


ops-status: ## Show local ops dashboard
	python3 scripts/ops/aladin_ops.py status

ops-errors: ## Show Loki errors; optional WINDOW=1h
	python3 scripts/ops/aladin_ops.py errors --window $(or $(WINDOW),15m)

ops-streams: ## Show provider stream status
	python3 scripts/ops/aladin_ops.py streams

ops-queues: ## Show Asynq/Redis queue counts
	python3 scripts/ops/aladin_ops.py queues

ops-force-stream: ## Force one stream due; requires PROVIDER=... STREAM_KEY="..."
	python3 scripts/ops/aladin_ops.py force-stream --provider "$(PROVIDER)" --stream-key "$(STREAM_KEY)"

ops-reset-stuck-cycles: ## Close stale active/running cycles; optional AGE=30m
	python3 scripts/ops/aladin_ops.py reset-stuck-cycles --age $(or $(AGE),30m)

# --- PROD stack ---------------------------------------------------------------
.PHONY: prod-env prod-check-env prod-build prod-up prod-down prod-restart prod-ps prod-logs prod-psql prod-backup prod-app prod-app-clear prod-app-uninstall

prod-env: ## Generate backend_v2/.env.prod (random infra passwords + OpenAI/Tavily from .env); FORCE=1 to regenerate
	bash scripts/ops/gen_prod_env.sh

prod-check-env:
	@test -f backend_v2/.env.prod || { echo ">> backend_v2/.env.prod missing — run 'make prod-env' first"; exit 1; }

prod-build: prod-check-env ## Build the prod backend + blocknote images (backend built once via `api`)
	$(PROD_COMPOSE) build api blocknote

prod-up: prod-build ## Build + start the prod stack (PROD_PROFILES selects processes)
	COMPOSE_PROFILES=$(PROD_PROFILES) $(PROD_COMPOSE) up -d
	@echo ">> prod up (profiles: $(PROD_PROFILES)). API http://localhost:8080  collab ws://localhost:3511  mcp http://localhost:8091"

prod-down: prod-check-env ## Stop the prod stack (ARGS=-v also drops prod volumes — DESTROYS DATA)
	COMPOSE_PROFILES=api,worker,mcp,collab $(PROD_COMPOSE) down $(ARGS)

prod-restart: prod-build ## Rebuild images + recreate prod services
	COMPOSE_PROFILES=$(PROD_PROFILES) $(PROD_COMPOSE) up -d --force-recreate

prod-ps: prod-check-env ## Show prod stack status
	COMPOSE_PROFILES=api,worker,mcp,collab $(PROD_COMPOSE) ps

prod-logs: prod-check-env ## Tail prod logs (SERVICE=api|worker|mcp|blocknote to scope)
	COMPOSE_PROFILES=api,worker,mcp,collab $(PROD_COMPOSE) logs -f $(SERVICE)

prod-psql: ## Open psql on the prod Postgres
	docker exec -it aladin-prod-postgres psql -U aladin -d aladin

prod-backup: ## Run a one-off prod Postgres backup (pg_dump -Fc, retained)
	bash scripts/ops/backup_prod.sh

prod-app: ## Build the desktop app pointed at prod + install it to /Applications (identifier com.aladin.app)
	cd aladin_react && VITE_DESKTOP_API_BASE_URL=http://localhost:8080 VITE_COLLAB_WS_URL=ws://localhost:3511 \
		npm run tauri:build -- --bundles app \
		--config '{"identifier":"com.aladin.app","productName":"Aladin"}'
	@echo ">> installing to /Applications/Aladin.app"
	rm -rf "/Applications/Aladin.app"
	cp -R "aladin_react/src-tauri/target/release/bundle/macos/Aladin.app" "/Applications/Aladin.app"
	@echo ">> installed. Launch it from /Applications (open -a Aladin)."

prod-app-clear: ## Wipe the prod app's LOCAL state (keep the app) — use after a prod DB wipe; FORCE=1 if it's running
	bash scripts/ops/prod_app_remove.sh clear

prod-app-uninstall: ## Remove /Applications/Aladin.app AND its local state (backend/notes untouched); FORCE=1 if running
	bash scripts/ops/prod_app_remove.sh uninstall

.PHONY: help backend mcp blocknote blocknote-test copilot-agent copilot-agent-test check-blocknote-versions tokens check-tokens nuke-local-db nuke-clients tauri-client-b db-up db-down test-db-up test-db-down test-go nango-up nango-down nango-logs env-nango ngrok-ensure worker-go api-go ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles ops-backfill-instruments ops-backfill-bars ops-backfill-corporate-actions

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
COPILOT_AGENT_ENV_KEYS = --key ANTHROPIC_API_KEY --key COPILOT_MODEL --key COPILOT_EFFORT --key COPILOT_AGENT_SHARED_SECRET --key ALADIN_MCP_URL --key COPILOT_AUTH

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
	# -p 1: the sandbox is ONE shared Postgres, and several outbox tests capture a global
	# horizon (pg_snapshot_xmin) or a shared-user snapshot, which flakes when packages run
	# concurrently. Serialize package test binaries so those integration tests are deterministic.
	cd backend_v2 && env -u DATABASE_URL TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test -p 1 ./...

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

ops-backfill-graph: ## Project the entity layer into Neo4j (the connection lens). Needs NEO4J_URI.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-graph

ops-backfill-instruments: ## Pull the Alpaca Assets universe into the instruments registry (T1). Needs ALPACA_API_KEY/SECRET.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-instruments

ops-backfill-bars: ## Pull historical daily bars from Alpaca into the bars store. Needs ALPACA_API_KEY/SECRET.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-bars

ops-backfill-corporate-actions: ## Pull splits/dividends from Alpaca into corporate_actions (adjust-on-read). Needs ALPACA_API_KEY/SECRET.
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && go run ./cmd/backfill-corporate-actions



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

# --- DEV app tier (the working tree, run in the background) ---------------------
# `make backend` still exists and runs the same services in the foreground with a
# tail of their logs; these targets are for when you want them up and your terminal
# back. They do not touch the infra containers — `make db-up` owns those.
.PHONY: dev-up dev-down dev-restart dev-status dev-logs dev-doctor dev-app dev-app-deps dev-help

dev-up: ## Start the dev tier (api :8000, mcp :8090, blocknote :3500/:3501, copilot :3550, worker, web :4173), killing whatever holds those ports first
	bash scripts/ops/run_dev.sh start

dev-down: ## Stop every dev service on those ports — including ones started by hand
	bash scripts/ops/run_dev.sh stop

dev-restart: ## dev-down + dev-up (rebuilds the Go binaries from the working tree)
	bash scripts/ops/run_dev.sh restart

dev-status: ## Show which dev services are up, on which port, and whether this tool started them
	@bash scripts/ops/run_dev.sh status

dev-logs: ## Tail the dev logs (SERVICE=api|mcp|blocknote|copilot-agent|worker|web to scope)
	bash scripts/ops/run_dev.sh logs

dev-doctor: ## Diagnose the dev loop: infra, config, processes, health, data
	@bash scripts/ops/dev_doctor.sh

dev-app-deps:
	@cd aladin_react && if [ ! -d node_modules ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then \
		echo ">> deps are stale (package-lock.json is newer than the installed tree) — npm ci"; \
		npm ci; \
	else \
		echo ">> deps up to date"; \
	fi

dev-app: dev-app-deps ## Run the desktop app from this working tree (tauri dev, hot reload). Frees :4173 first — tauri starts its own vite
	@PROCS=web bash scripts/ops/run_dev.sh stop
	cd aladin_react && npm run tauri:dev

dev-help: ## Explain the dev commands: what to run, what each one touches, in what order
	@printf '%b\n' \
	  '' \
	  '\033[1mDEV\033[0m — everything runs from the \033[1mworking tree\033[0m. No releases, no backups.' \
	  '' \
	  '\033[1m  Daily\033[0m' \
	  '    make dev-doctor           diagnose it all: infra, config, processes, health, data' \
	  '    make dev-up               start the app tier in the background' \
	  '    make dev-restart          rebuild from the working tree and start over' \
	  '    make dev-status           what is up, on which port, and who started it' \
	  '    make dev-logs             tail .dev/logs/          SERVICE=api|worker|web|...' \
	  '    make dev-down             stop everything on the dev ports' \
	  '' \
	  '\033[1m  What dev-up runs\033[0m   PROCS="api web" scopes both start and stop' \
	  '    api :8000 · mcp :8090 · blocknote :3500 + collab :3501 · copilot-agent :3550' \
	  '    worker (no port) · web :4173' \
	  '    It is NOT additive: whatever holds a dev port is killed first, so a stale' \
	  '    process cannot keep serving old code on a port you thought you restarted.' \
	  '    A port held by a CONTAINER is left alone — stop that with docker compose.' \
	  '' \
	  '\033[1m  Foreground alternative\033[0m' \
	  '    make backend              api + blocknote + copilot-agent, logs in your terminal' \
	  '    make worker-go · make mcp · (cd aladin_react && npm run dev)' \
	  '    Same services. dev-up exists for when you want your terminal back.' \
	  '' \
	  '\033[1m  Infra\033[0m   Docker: postgres :5433, redis :6379, neo4j :7687 — dev-up never touches these' \
	  '    make db-up · make db-down          the containers the app tier connects to' \
	  '    make test-db-up                    the ISOLATED sandbox (pg :5444) — tests only' \
	  '' \
	  '\033[1m  Clients\033[0m' \
	  '    make dev-app              run the desktop app from this tree (tauri dev, hot reload)' \
	  '    make dev-ipad             iPad companion "Anchor Dev" -> the connected iPad' \
	  '    make tauri-client-b       a 2nd desktop client with its own login, to test collab' \
	  '' \
	  '\033[1m  Resetting\033[0m' \
	  '    make nuke-local-db        wipe ONE Tauri client local SQLite after a schema change' \
	  '    make nuke-clients         wipe every clients local SQLite (after a server DB reset)' \
	  '' \
	  '  make prod-help  is the same map for prod.  make help  lists every target.' \
	  ''

# --- PROD stack ---------------------------------------------------------------
.PHONY: prod-env prod-check-env prod-build prod-up prod-down prod-restart prod-ps prod-logs prod-psql prod-backup prod-backup-install prod-backup-status prod-restore-drill prod-run prod-run-stop prod-run-status prod-update prod-doctor prod-help prod-nuke prod-release prod-release-list prod-release-clean prod-release-version prod-app prod-app-deps prod-app-clear prod-app-uninstall prod-ipad dev-ipad

PROD_BACKUP_INSTALL_DIR := $(HOME)/Library/Application Support/aladin

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

prod-backup-install: ## Install/refresh the nightly 03:00 backup LaunchAgent (re-run after editing backup_prod.sh)
	@# The script is COPIED out of the repo on purpose: ~/Documents is TCC-protected
	@# and a launchd agent reading from there fails with exit 126, silently never
	@# producing a dump. See scripts/ops/com.aladin.prod.backup.plist.
	mkdir -p "$(PROD_BACKUP_INSTALL_DIR)"
	cp scripts/ops/backup_prod.sh "$(PROD_BACKUP_INSTALL_DIR)/backup_prod.sh"
	chmod +x "$(PROD_BACKUP_INSTALL_DIR)/backup_prod.sh"
	mkdir -p "$(HOME)/aladin-backups"
	cp scripts/ops/com.aladin.prod.backup.plist "$(HOME)/Library/LaunchAgents/"
	-launchctl bootout gui/$(shell id -u)/com.aladin.prod.backup 2>/dev/null
	launchctl bootstrap gui/$(shell id -u) "$(HOME)/Library/LaunchAgents/com.aladin.prod.backup.plist"
	@echo ">> installed. Verifying it can actually run under launchd..."
	launchctl kickstart -k gui/$(shell id -u)/com.aladin.prod.backup
	@sleep 8
	@launchctl print gui/$(shell id -u)/com.aladin.prod.backup | grep -E 'last exit code' \
		| grep -q 'last exit code = 0' \
		&& echo ">> OK — launchd ran the backup successfully (nightly 03:00)." \
		|| { echo ">> FAILED — check ~/aladin-backups/backup.log"; exit 1; }

prod-backup-status: ## Show the backup agent's state + the dumps on disk
	@launchctl print gui/$(shell id -u)/com.aladin.prod.backup 2>/dev/null \
		| grep -E 'state|runs|last exit code' || echo ">> agent not installed — run 'make prod-backup-install'"
	@echo "--- dumps in $(HOME)/aladin-backups ---"
	@ls -lht "$(HOME)/aladin-backups"/aladin-prod-*.dump 2>/dev/null | head -5 || echo ">> no dumps yet"

prod-restore-drill: ## Prove the newest dump restores (into a throwaway DB; never touches `aladin`)
	bash scripts/ops/restore_drill.sh

# --- native prod app tier (Go + node sidecars run as host processes) ----------
# Only the DATA tier stays in Docker. See PROD.md "Native release".
prod-release: ## Build a native prod release from a git ref (REF=main; NO_SWITCH=1 to not flip `current`)
	REF=$(or $(REF),main) bash scripts/ops/build_prod_release.sh

prod-release-list: ## List built releases and which one `current` points at
	@P="$(HOME)/Library/Application Support/aladin"; \
	if C=$$(readlink "$$P/current" 2>/dev/null) && [ -n "$$C" ]; then \
		echo "current -> $$(basename "$$C")"; \
	else echo "current -> (none)"; fi; \
	echo "--- releases ---"; ls -1t "$$P/releases" 2>/dev/null || echo "(none built)"

prod-run: ## Start the current release's app tier, killing any processes from a previous release first
	bash scripts/ops/run_prod_release.sh start

prod-run-stop: ## Stop every process running from any release
	bash scripts/ops/run_prod_release.sh stop

prod-run-status: ## Show which release each running process came from (flags stale ones)
	@bash scripts/ops/run_prod_release.sh status

prod-nuke: ## Remove the LOCAL prod install (containers+volumes, native install, agent, app). Keeps ~/aladin-backups and .env.prod. DRY_RUN=1 to preview
	@bash scripts/ops/prod_nuke.sh

prod-update: ## THE DEPLOY: build a release (REF=main), back up + drill, activate, verify
	@bash scripts/ops/prod_update.sh

prod-doctor: ## Diagnose the whole prod stack: data tier, processes, health, backups, disk
	@bash scripts/ops/prod_doctor.sh

prod-help: ## Explain the prod commands: what to run, what each one touches, in what order
	@printf '%b\n' \
	  '' \
	  '\033[1mPROD\033[0m — the app tier runs \033[1mnatively\033[0m; the data tier runs in Docker.' \
	  '' \
	  '\033[1m  Daily\033[0m' \
	  '    make prod-doctor          diagnose it all: data tier, processes, health, backups, disk' \
	  '    make prod-update          THE DEPLOY (backend): build -> back up + drill -> activate -> verify' \
	  '    make prod-app             build + install the DESKTOP APP to /Applications' \
	  '' \
	  '\033[1m  Those two are separate on purpose\033[0m' \
	  '    prod-update  ->  api, worker, mcp, blocknote, copilot-agent  (Go + node, from a git archive)' \
	  '    prod-app     ->  the Tauri client — the ONLY way frontend code ships' \
	  '    Neither builds the other. A frontend-only change needs prod-app, not prod-update.' \
	    '    prod-app runs npm ci first when package-lock.json is newer than node_modules.' \
	  '' \
	  '\033[1m  Migrations\033[0m' \
	  '    The api applies pending goose migrations ON BOOT. One-way — there is no down step.' \
	  '    That is why prod-update backs up and PROVES the dump restores before it activates,' \
	  '    and refuses to migrate if either fails.   override: SKIP_DRILL=1 · SKIP_BACKUP=1' \
	  '    prod-doctor reports the schema version the database is currently at.' \
	  '' \
	  '\033[1m  Data safety\033[0m' \
	  '    make prod-backup          one-off dump + file archive, verified as a pair and retained' \
	  '    make prod-restore-drill   prove the newest dump restores, into a throwaway DB' \
	  '    make prod-backup-install  install/refresh the nightly 03:00 LaunchAgent' \
	  '    make prod-backup-status   agent state + what is on disk' \
	  '' \
	  '\033[1m  Releases\033[0m   ~/Library/Application Support/aladin/releases/<stamp>-<sha>' \
	  '    make prod-release         build one from a ref            REF=main · NO_SWITCH=1' \
	  '    make prod-release-list    list them, and which is `current`' \
	  '    make prod-run-status      which release each live process came from (flags stale ones)' \
	  '    make prod-release-clean   prune old ones                  KEEP=3 · DRY_RUN=1' \
	  '' \
	  '\033[1m  Processes\033[0m' \
	  '    make prod-run             start the current release, killing older-release processes first' \
	  '    make prod-run-stop        stop every process from any release' \
	  '' \
	  '\033[1m  Data tier\033[0m   Docker: postgres :5455, redis — the native release connects over those ports' \
	  '    make prod-ps              container status' \
	  '    make prod-psql            psql on the prod database' \
	  '    make prod-logs            container logs        SERVICE=api|worker|mcp|blocknote' \
	  '' \
	  '\033[1m  Destructive\033[0m' \
	  '    make prod-down ARGS=-v    DROPS THE PROD VOLUMES' \
	  '    make prod-nuke            remove the local install; keeps ~/aladin-backups + .env.prod  DRY_RUN=1' \
	  '    make prod-app-uninstall   remove the app AND its local state (backend untouched)' \
	  '' \
	  '  make help  lists every target, prod and otherwise.' \
	  ''

prod-release-clean: ## Prune old releases (KEEP=3; DRY_RUN=1 to preview). Never removes `current` or one with a live process
	KEEP=$(or $(KEEP),3) bash scripts/ops/clean_prod_releases.sh

prod-release-version: ## Show the VERSION stamp of the current release
	@cat "$(HOME)/Library/Application Support/aladin/current/VERSION" 2>/dev/null \
		|| echo ">> no current release — run 'make prod-release'"

prod-app-deps:
	@cd aladin_react && if [ ! -d node_modules ] || [ package-lock.json -nt node_modules/.package-lock.json ]; then \
		echo ">> deps are stale (package-lock.json is newer than the installed tree) — npm ci"; \
		npm ci; \
	else \
		echo ">> deps up to date"; \
	fi

prod-app: prod-app-deps ## Build the desktop app pointed at prod + install it to /Applications (identifier com.aladin.app)
	cd aladin_react && VITE_DESKTOP_API_BASE_URL=http://localhost:8080 VITE_COLLAB_WS_URL=ws://localhost:3511 \
		VITE_BOARD_SYNC_WS_URL=ws://localhost:3512 \
		npm run tauri:build -- --bundles app \
		--config '{"identifier":"com.aladin.app","productName":"Aladin"}'
	@echo ">> installing to /Applications/Aladin.app"
	rm -rf "/Applications/Aladin.app"
	cp -R "aladin_react/src-tauri/target/release/bundle/macos/Aladin.app" "/Applications/Aladin.app"
	@echo ">> installed. Launch it from /Applications (open -a Aladin)."

prod-ipad: ## Install "Anchor" on the connected iPad: Release build pointed at PROD (:8080/:3511). HOST=… DEVICE=… NO_LAUNCH=1
	bash scripts/ops/ipad_install.sh prod

dev-ipad: ## Install "Anchor Dev" alongside it: same build pointed at the DEV stack (:8000/:3501); own id, own local DB
	bash scripts/ops/ipad_install.sh dev

prod-app-clear: ## Wipe the prod app's LOCAL state (keep the app) — use after a prod DB wipe; FORCE=1 if it's running
	bash scripts/ops/prod_app_remove.sh clear

prod-app-uninstall: ## Remove /Applications/Aladin.app AND its local state (backend/notes untouched); FORCE=1 if running
	bash scripts/ops/prod_app_remove.sh uninstall

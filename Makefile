.PHONY: help backend mcp blocknote blocknote-test check-blocknote-versions nuke-local-db nuke-clients tauri-client-b db-up db-down test-db-up test-db-down test-go nango-up nango-down nango-logs env-nango ngrok-ensure worker-go api-go artifact-spa-build ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles

# --- Isolated sandbox stack (docker-compose.test.yml) -----------------------
# A throwaway mirror of the dev infra on DISTINCT ports, namespaced under the
# `aladin-test` project, used for agent/automated testing so the real `aladin-*`
# dev stack and its data are never touched.
TEST_COMPOSE := docker compose -f docker-compose.test.yml -p aladin-test
TEST_DATABASE_URL := postgres://aladin:password@localhost:5444/aladin

help: ## List available make targets
	@awk 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

backend: ngrok-ensure ## Run the blocknote sidecar (local) + the Go API on :8000 (needs infra up; see `make db-up`)
	@(cd services/blocknote && exec node server.js) & \
	SIDECAR_PID=$$!; \
	trap 'kill $$SIDECAR_PID 2>/dev/null' EXIT INT TERM; \
	echo ">> blocknote sidecar pid $$SIDECAR_PID (converter :3500, collab :3501)"; \
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && API_ADDR=:8000 go run ./cmd/api

mcp: ## Run the MCP page server on port 8090
	eval "$$(python3 scripts/ops/read_env_keys.py --env backend_v2/.env)" && cd backend_v2 && MCP_HTTP_ADDR=:8090 go run ./cmd/mcp

blocknote: ## Run the blocknote Node sidecar locally (converter :3500 + collab :3501); needs postgres + Go API up
	cd services/blocknote && npm start

blocknote-test: ## Run the blocknote Node service unit tests
	cd services/blocknote && npm test

check-blocknote-versions: ## Fail if @blocknote/* + yjs versions drift between aladin_react and services/blocknote
	bash scripts/check-blocknote-versions.sh

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
	cd backend_v2 && go run ./cmd/api

artifact-spa-build: ## Build and copy the React artifact editor bundle
	cd aladin_ui/composeApp/react-spa && npm install && npm run build
	rm -rf aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa
	mkdir -p aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa
	cp aladin_ui/composeApp/react-spa/dist/artifact-spa.js aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa/artifact-spa.js
	cp aladin_ui/composeApp/react-spa/dist/style.css aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa/style.css

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

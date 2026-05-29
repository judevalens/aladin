.PHONY: help backend mcp blocknote blocknote-test check-blocknote-versions nuke-local-db tauri-client-b db-up db-down nango-up nango-down nango-logs env-nango ngrok-ensure worker-go api-go artifact-spa-build ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles

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

tauri-client-b: ## Build + launch a 2nd Tauri client (own identity/login) to simulate a second collab user (macOS)
	@echo ">> building a second Tauri client (id com.aladin.react.b); first build takes a few minutes..."
	cd aladin_react && npm run tauri:build -- --debug --bundles app --config '{"identifier":"com.aladin.react.b","productName":"AladinB"}'
	open "aladin_react/src-tauri/target/debug/bundle/macos/AladinB.app"

db-up: ## Start local Docker infrastructure
	docker compose up -d

db-down: ## Stop local Docker infrastructure
	docker compose down

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

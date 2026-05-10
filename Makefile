.PHONY: help backend db-up db-down nango-up nango-down nango-logs worker-go api-go artifact-spa-build ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles

help: ## List available make targets
	@awk 'BEGIN {FS = ":.*## "; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

backend: ## Run the Go backend API on port 8000
	cd backend_v2 && API_ADDR=:8000 go run ./cmd/api

db-up: ## Start local Docker infrastructure
	docker compose up -d

db-down: ## Stop local Docker infrastructure
	docker compose down

nango-up: ## Start local Nango self-hosted services
	docker compose -f docker-compose.nango.yml up -d

nango-down: ## Stop local Nango self-hosted services
	docker compose -f docker-compose.nango.yml down

nango-logs: ## Tail local Nango logs
	docker compose -f docker-compose.nango.yml logs -f

worker-go: ## Run the Go worker; optional CONCURRENCY=24
	cd backend_v2 && WORKER_CONCURRENCY=$(or $(CONCURRENCY),16) go run ./cmd/worker

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

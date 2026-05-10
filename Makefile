.PHONY: dev backend frontend install db-up db-down db-migrate db-revision worker sync-reddit sync-twitter sync-insights worker-go api-go artifact-spa-build ops-status ops-errors ops-streams ops-queues ops-force-stream ops-reset-stuck-cycles

dev:
	@$(MAKE) -j2 backend frontend

backend:
	cd backend_v2 && API_ADDR=:8000 go run ./cmd/api

frontend:
	cd frontend && npm run dev

install:
	cd backend && poetry install
	cd frontend && npm install

db-up:
	docker compose up -d

db-down:
	docker compose down

db-migrate:
	cd backend && poetry run alembic upgrade head

db-revision:
	cd backend && poetry run alembic revision --autogenerate -m "$(msg)"

worker:
	cd backend && poetry run python run_worker.py

sync-reddit:
	cd backend && poetry run python run_sync.py reddit

sync-twitter:
	cd backend && poetry run python run_sync.py twitter

sync-insights:
	cd backend && poetry run python -c "from dotenv import load_dotenv; load_dotenv(); from app.pipeline.insight_worker import InsightWorker; InsightWorker().run_once()"

worker-go:
	cd backend_v2 && WORKER_CONCURRENCY=$(or $(CONCURRENCY),16) go run ./cmd/worker

api-go:
	cd backend_v2 && go run ./cmd/api

artifact-spa-build:
	cd aladin_ui/composeApp/react-spa && npm install && npm run build
	rm -rf aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa
	mkdir -p aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa
	cp aladin_ui/composeApp/react-spa/dist/artifact-spa.js aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa/artifact-spa.js
	cp aladin_ui/composeApp/react-spa/dist/style.css aladin_ui/composeApp/src/wasmJsMain/resources/artifact-spa/style.css

ops-status:
	python3 scripts/ops/aladin_ops.py status

ops-errors:
	python3 scripts/ops/aladin_ops.py errors --window $(or $(WINDOW),15m)

ops-streams:
	python3 scripts/ops/aladin_ops.py streams

ops-queues:
	python3 scripts/ops/aladin_ops.py queues

ops-force-stream:
	python3 scripts/ops/aladin_ops.py force-stream --provider "$(PROVIDER)" --stream-key "$(STREAM_KEY)"

ops-reset-stuck-cycles:
	python3 scripts/ops/aladin_ops.py reset-stuck-cycles --age $(or $(AGE),30m)

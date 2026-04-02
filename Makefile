.PHONY: dev backend frontend install db-up db-down db-migrate db-revision worker sync-reddit sync-twitter sync-insights worker-go

dev:
	@$(MAKE) -j2 backend frontend

backend:
	cd backend && poetry run flask --app app run --debug --port 8000

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
	cd backend_v2 && go run ./cmd/worker

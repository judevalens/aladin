.PHONY: dev backend frontend install db-up db-down db-migrate db-revision

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

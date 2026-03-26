# Aladin — Research Assistant Knowledge Graph

## Structure

```
aladin/
├── backend/        # Python / Flask API
└── frontend/       # React + Vite + TypeScript + Tailwind
```

## Getting Started

### Backend
```bash
cd backend
poetry install
poetry run flask --app app run --debug
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

### Run both together (from root)
```bash
make install   # install all dependencies
make dev       # start backend + frontend concurrently
```

# dbdiagram clone

DBML editor with a live ER diagram. Go + Postgres backend, React frontend.

```
backend/    Go modular monolith (gin, sqlx, squirrel, golang-migrate, swagger)  -> backend/AGENTS.md
frontend/   React + Vite + TypeScript (zustand, React Flow, Monaco)              -> frontend/AGENTS.md
docs/       API-CONTRACT.md — the shared HTTP contract
```

Both follow the layering of `my_fidani` (`myfidani_go_backend`, `myfidani_react_admin`).

## Run

```sh
createdb dbdiagram
cd backend
cp .env.example .env        # set POSTGRES_USER and JWT_SECRET (openssl rand -hex 32)
make migrate-up             # schemas shared + dbdiagram
make run                    # http://localhost:8080  (swagger: /swagger/index.html)
# first time only, after you have registered in the UI: take over diagrams made before accounts existed
make claim email=you@example.com

cd ../frontend
cp .env.example .env
npm install && npm run dev
```

## Check

```sh
cd backend  && make verify            # build, vet, race tests (no DB)
cd backend  && createdb dbdiagram_test && make test   # + Postgres suite
cd frontend && npm run typecheck && npm run lint && npm test && npm run build
```

## Production Deployment (Docker Compose)

The project includes ready-to-use Docker configs (`postgres`, `backend`, `frontend`, and optional `caddy` for automatic SSL).

1. Copy `.env.example` and set production secrets:
```sh
cp .env.example .env
# Fill DOMAIN, POSTGRES_PASSWORD, and generate JWT_SECRET (openssl rand -hex 32)
```

2. Run with standard HTTP (port 80 or `$APP_PORT`):
```sh
./deploy.sh
# or: docker compose up -d --build
```

3. Run with automatic HTTPS (Let's Encrypt SSL via Caddy):
```sh
./deploy.sh --ssl
# or: docker compose --profile ssl up -d --build
```

## Features

Accounts (email + password, rotating refresh tokens); nested folders shared with other people as owner / editor / viewer (a folder share covers everything inside); edit-conflict detection with live refresh; diagram CRUD with autosave, version history (automatic snapshots, manual save, restore), folders, public read-only share links, DBML editor with
error markers, draggable tables with saved positions, SQL import, export to DBML / SQL /
PNG / PDF. Not built yet: email verification, password reset, real-time (websocket) collaboration.

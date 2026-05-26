# Flask API Blueprint

Flask 3 + SQLAlchemy 2 + Postgres backend with a `/healthz` probe and
an `/items` CRUD surface. Ready to ship behind gunicorn.

## Quick start

```bash
cp .env.example .env
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
flask --app app run --port 8000
```

## Endpoints

| Method | Path             | Purpose                  |
|--------|------------------|--------------------------|
| GET    | `/healthz`       | Liveness probe (no DB)   |
| GET    | `/items`         | List items               |
| POST   | `/items`         | Create item              |
| GET    | `/items/<id>`    | Fetch a single item      |
| PUT    | `/items/<id>`    | Update title/body        |
| DELETE | `/items/<id>`    | Delete an item           |

Example:

```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"title": "first item", "body": "hello"}' \
  http://localhost:8000/items
```

## Docker

```bash
docker build -t ironflyer-flask-api .
docker run --rm -p 8000:8000 --env-file .env ironflyer-flask-api
```

## Migrations

`SQLAlchemy` auto-creates tables on first run (`db.create_all`). For
production, prefer running the SQL in `migrations/001_init.sql`
manually or wiring Alembic.

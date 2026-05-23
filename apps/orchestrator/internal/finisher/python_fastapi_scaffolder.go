// PythonFastAPIScaffolder — FastAPI + SQLModel + asyncpg skeleton for
// Python-flavored backend services (typical use: ML / AI pipelines or
// teams that already speak Python). Triggers when the stack names
// python + (api|backend|fastapi), or when stories mention ml,
// ai pipeline, or fastapi.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type PythonFastAPIScaffolder struct{}

func (PythonFastAPIScaffolder) Name() string { return "python-fastapi" }

func (PythonFastAPIScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	hasPython := strings.Contains(stack, "python") || strings.Contains(stack, "fastapi") ||
		strings.Contains(stack, "django") || strings.Contains(stack, "flask")
	hasServerHint := strings.Contains(stack, "api") || strings.Contains(stack, "backend") ||
		strings.Contains(stack, "fastapi") || strings.Contains(stack, "service")
	if hasPython && hasServerHint {
		return true
	}
	if strings.Contains(stack, "fastapi") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "fastapi") || strings.Contains(body, "ai pipeline") ||
			strings.Contains(body, "ml pipeline") || strings.Contains(body, "ml model") ||
			strings.Contains(body, "machine learning") {
			return true
		}
		if hasPython && strings.Contains(body, " ml ") {
			return true
		}
	}
	return false
}

// pyProjectName turns a free-form project name into a PEP 503
// normalized distribution name (lowercase, hyphen-separated).
func pyProjectName(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "ironflyer-service"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "ironflyer-service"
	}
	return out
}

func (PythonFastAPIScaffolder) Scaffold(_ context.Context, p *domain.Project) (DomainScaffold, error) {
	projectName := "ironflyer-service"
	if p != nil {
		projectName = pyProjectName(p.Name)
	}

	pyproject := "" +
		"# uv-compatible project layout. Install with `uv sync` (or\n" +
		"# `pip install -e .`); run with `uvicorn app.main:app --reload`.\n" +
		"[project]\n" +
		"name = \"" + projectName + "\"\n" +
		"version = \"0.1.0\"\n" +
		"description = \"FastAPI service scaffolded by Ironflyer\"\n" +
		"requires-python = \">=3.12\"\n" +
		"dependencies = [\n" +
		"  \"fastapi>=0.115,<0.116\",\n" +
		"  \"uvicorn[standard]>=0.32,<0.33\",\n" +
		"  \"sqlmodel>=0.0.22\",\n" +
		"  \"asyncpg>=0.30,<0.31\",\n" +
		"  \"pydantic>=2.9,<3.0\",\n" +
		"  \"pydantic-settings>=2.5\",\n" +
		"  \"python-dotenv>=1.0\",\n" +
		"]\n" +
		"\n" +
		"[build-system]\n" +
		"requires = [\"hatchling>=1.25\"]\n" +
		"build-backend = \"hatchling.build\"\n" +
		"\n" +
		"[tool.hatch.build.targets.wheel]\n" +
		"packages = [\"app\"]\n"

	mainPy := "" +
		"# FastAPI entrypoint. Wires the lifespan (DB engine up/down),\n" +
		"# mounts the routers, and exposes /health + /version. Keep this\n" +
		"# file thin: domain logic lives in app/routers and app/services.\n" +
		"from contextlib import asynccontextmanager\n" +
		"\n" +
		"from dotenv import load_dotenv\n" +
		"from fastapi import FastAPI\n" +
		"\n" +
		"from app.db import dispose_engine, init_engine\n" +
		"from app.routers import users\n" +
		"\n" +
		"load_dotenv(\".env.local\")\n" +
		"load_dotenv(\".env\")\n" +
		"\n" +
		"\n" +
		"@asynccontextmanager\n" +
		"async def lifespan(_: FastAPI):\n" +
		"    await init_engine()\n" +
		"    try:\n" +
		"        yield\n" +
		"    finally:\n" +
		"        await dispose_engine()\n" +
		"\n" +
		"\n" +
		"app = FastAPI(title=\"Ironflyer Service\", version=\"0.1.0\", lifespan=lifespan)\n" +
		"app.include_router(users.router, prefix=\"/users\", tags=[\"users\"])\n" +
		"\n" +
		"\n" +
		"@app.get(\"/health\")\n" +
		"async def health() -> dict[str, str]:\n" +
		"    return {\"status\": \"ok\"}\n" +
		"\n" +
		"\n" +
		"@app.get(\"/version\")\n" +
		"async def version() -> dict[str, str]:\n" +
		"    return {\"version\": app.version}\n"

	usersPy := "" +
		"# Example async router. Demonstrates the dependency-injection\n" +
		"# pattern (get_session) and SQLModel querying. Add new endpoints\n" +
		"# here or split into a sub-package once this file grows.\n" +
		"from fastapi import APIRouter, Depends, HTTPException, status\n" +
		"from sqlalchemy import select\n" +
		"from sqlmodel.ext.asyncio.session import AsyncSession\n" +
		"\n" +
		"from app.db import get_session\n" +
		"from app.models import User\n" +
		"\n" +
		"router = APIRouter()\n" +
		"\n" +
		"\n" +
		"@router.get(\"/{user_id}\")\n" +
		"async def get_user(user_id: str, session: AsyncSession = Depends(get_session)) -> dict[str, str]:\n" +
		"    result = await session.execute(select(User).where(User.id == user_id))\n" +
		"    user = result.scalar_one_or_none()\n" +
		"    if user is None:\n" +
		"        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail=\"not found\")\n" +
		"    return {\"id\": user.id, \"email\": user.email}\n"

	dbPy := "" +
		"# Async SQLModel engine + session dependency. The engine is\n" +
		"# created on app startup (lifespan) and disposed on shutdown so\n" +
		"# tests and reload-mode don't leak connections.\n" +
		"import os\n" +
		"from typing import AsyncIterator\n" +
		"\n" +
		"from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine\n" +
		"from sqlmodel.ext.asyncio.session import AsyncSession\n" +
		"\n" +
		"_engine: AsyncEngine | None = None\n" +
		"\n" +
		"\n" +
		"def _database_url() -> str:\n" +
		"    raw = os.getenv(\"DATABASE_URL\")\n" +
		"    if not raw:\n" +
		"        raise RuntimeError(\n" +
		"            \"DATABASE_URL is required; the DB scaffold writes it to .env.local\"\n" +
		"        )\n" +
		"    # SQLAlchemy's async dialect needs the +asyncpg suffix.\n" +
		"    if raw.startswith(\"postgres://\"):\n" +
		"        raw = \"postgresql+asyncpg://\" + raw[len(\"postgres://\") :]\n" +
		"    elif raw.startswith(\"postgresql://\") and \"+asyncpg\" not in raw:\n" +
		"        raw = \"postgresql+asyncpg://\" + raw[len(\"postgresql://\") :]\n" +
		"    return raw\n" +
		"\n" +
		"\n" +
		"async def init_engine() -> None:\n" +
		"    global _engine\n" +
		"    _engine = create_async_engine(_database_url(), pool_pre_ping=True, future=True)\n" +
		"\n" +
		"\n" +
		"async def dispose_engine() -> None:\n" +
		"    global _engine\n" +
		"    if _engine is not None:\n" +
		"        await _engine.dispose()\n" +
		"        _engine = None\n" +
		"\n" +
		"\n" +
		"async def get_session() -> AsyncIterator[AsyncSession]:\n" +
		"    if _engine is None:\n" +
		"        raise RuntimeError(\"engine not initialized; lifespan did not run\")\n" +
		"    async with AsyncSession(_engine, expire_on_commit=False) as session:\n" +
		"        yield session\n"

	modelsPy := "" +
		"# SQLModel User example. Mirror this shape for new entities;\n" +
		"# SQLModel doubles as both the ORM mapping and the Pydantic\n" +
		"# schema, so handlers can return models directly.\n" +
		"from datetime import datetime\n" +
		"from typing import Optional\n" +
		"\n" +
		"from sqlmodel import Field, SQLModel\n" +
		"\n" +
		"\n" +
		"class User(SQLModel, table=True):\n" +
		"    __tablename__ = \"users\"\n" +
		"\n" +
		"    id: str = Field(primary_key=True)\n" +
		"    email: str = Field(index=True, unique=True)\n" +
		"    display_name: Optional[str] = None\n" +
		"    created_at: datetime = Field(default_factory=datetime.utcnow)\n"

	initPy := "# app package marker.\n"
	routersInitPy := "# routers sub-package marker.\n"

	dockerfile := "" +
		"# Multi-stage: install deps in a builder layer, copy them into a\n" +
		"# slim runtime so the final image stays small.\n" +
		"FROM python:3.12-slim AS builder\n" +
		"ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PIP_NO_CACHE_DIR=1\n" +
		"WORKDIR /app\n" +
		"RUN apt-get update && apt-get install -y --no-install-recommends build-essential libpq-dev && rm -rf /var/lib/apt/lists/*\n" +
		"COPY pyproject.toml ./\n" +
		"COPY app ./app\n" +
		"RUN pip install --upgrade pip && pip install --prefix=/install .\n" +
		"\n" +
		"FROM python:3.12-slim AS runtime\n" +
		"ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1\n" +
		"WORKDIR /app\n" +
		"RUN apt-get update && apt-get install -y --no-install-recommends libpq5 && rm -rf /var/lib/apt/lists/*\n" +
		"COPY --from=builder /install /usr/local\n" +
		"COPY app ./app\n" +
		"EXPOSE 8080\n" +
		"CMD [\"uvicorn\", \"app.main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8080\"]\n"

	dockerignore := "" +
		"__pycache__\n" +
		"*.pyc\n" +
		".venv\n" +
		"venv\n" +
		".env\n" +
		".env.local\n" +
		".git\n" +
		"*.md\n" +
		"Dockerfile\n" +
		".dockerignore\n" +
		"dist\n" +
		"build\n" +
		"*.egg-info\n"

	gitignore := "" +
		"__pycache__/\n" +
		"*.pyc\n" +
		".venv/\n" +
		"venv/\n" +
		".env\n" +
		".env.local\n" +
		"dist/\n" +
		"build/\n" +
		"*.egg-info/\n" +
		".pytest_cache/\n" +
		".mypy_cache/\n"

	files := map[string]string{
		"pyproject.toml":          pyproject,
		"app/__init__.py":         initPy,
		"app/main.py":             mainPy,
		"app/db.py":               dbPy,
		"app/models.py":           modelsPy,
		"app/routers/__init__.py": routersInitPy,
		"app/routers/users.py":    usersPy,
		"Dockerfile":              dockerfile,
		".dockerignore":           dockerignore,
		".gitignore":              gitignore,
	}

	contract := "Python service scaffold: FastAPI + SQLModel + asyncpg + uvicorn.\n" +
		"\n" +
		"Already provisioned:\n" +
		"- pyproject.toml          uv-compatible, project=" + projectName + ", pinned deps\n" +
		"- app/main.py             FastAPI() instance, lifespan, /health /version\n" +
		"- app/routers/users.py    example async router under /users\n" +
		"- app/db.py               async engine + get_session dependency\n" +
		"- app/models.py           SQLModel User example\n" +
		"- Dockerfile              python:3.12-slim builder -> python:3.12-slim runtime\n" +
		"- .dockerignore           keeps venv + .env + caches out of context\n" +
		"- .gitignore              ignores __pycache__, .venv, .env*, build artifacts\n" +
		"\n" +
		"Run locally with `uvicorn app.main:app --reload`. DATABASE_URL is\n" +
		"required; the Supabase or SharedPostgres provisioner writes it to\n" +
		".env.local and python-dotenv loads it on import. The db module\n" +
		"rewrites postgres:// to postgresql+asyncpg:// automatically so\n" +
		"plain DSNs from the provisioner work without extra glue.\n" +
		"\n" +
		"Layout rules: keep main.py thin; routers go under app/routers/\n" +
		"and are mounted in main.py; models stay in app/models.py until\n" +
		"the count grows past ~10, then split per-domain. Use the\n" +
		"get_session dependency for every DB-touching endpoint.\n"

	return DomainScaffold{Files: files, Contract: contract}, nil
}

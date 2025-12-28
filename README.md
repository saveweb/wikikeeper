# WikiKeeper

Wiki statistics tracker and Archive.org backup status checker.

## Features

- 📊 Track MediaWiki site statistics over time
- 📦 Check Archive.org for existing backups
- 🚀 Fast API built with FastAPI + httpx
- 💾 MongoDB + Beanie ODM
- 📝 Comprehensive logging with loguru
- ⚡ Managed with uv for lightning-fast dependency management

## Quick Start

### Development

```bash
# Install uv (if not already installed)
curl -LsSf https://astral.sh/uv/install.sh | sh

# Install dependencies
uv sync

# Copy environment file
cp .env.example .env

# Start MongoDB (Docker)
docker-compose up -d mongodb

# Run development server
uv run python -m wikikeeper.app.main

# API will be available at http://localhost:8000
# API docs at http://localhost:8000/docs
```

### Production (Docker)

```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Requirements

- Python 3.13+
- MongoDB 8+
- uv (package manager)

## API Endpoints

- `GET /` - API info
- `GET /health` - Health check
- `GET /api/wikis` - List wikis
- `POST /api/wikis` - Add new wiki
- `GET /api/wikis/{id}` - Get wiki details
- `POST /api/wikis/{id}/check` - Trigger stats collection
- `GET /api/wikis/{id}/stats` - Get historical stats
- `GET /api/wikis/{id}/archives` - Get archive info
- `POST /api/wikis/{id}/check-archive` - Check Archive.org
- `GET /api/stats/summary` - Overall statistics

## Architecture

```
wikikeeper/
├── src/wikikeeper/
│   ├── app/           # FastAPI application
│   ├── core/          # Config, exceptions, logging
│   ├── db/            # Database connection
│   ├── models/        # Beanie ODM models
│   └── services/      # Business logic
│       ├── mediawiki.py       # MediaWiki API client (httpx)
│       ├── archive_checker.py # Archive.org checker
│       └── collector.py        # Data collector
├── tests/             # Unit tests
├── logs/              # Application logs
└── docker-compose.yml
```

## Implementation Notes

### MediaWiki API Client
- Uses **httpx** (async) instead of requests
- Fetches siteinfo (statistics + general info)
- Reference: wikiteam3 implementation (not used as dependency)
- API: https://www.mediawiki.org/wiki/API:Siteinfo

### Archive.org Checker
- Uses **internetarchive** library for search
- Uses **httpx** for metadata fetching
- Reference: wikiapiary-wikiteam-bot (not used as dependency)
- Docs: https://archive.org/help/aboutsearch.htm

### Database
- MongoDB with Beanie ODM
- Time-series data for statistics
- Separate collection for Archive.org metadata

## Development

```bash
# Install dependencies
uv sync

# Run with dev dependencies
uv sync --dev

# Run tests
uv run pytest

# Run with coverage
uv run pytest --cov=wikikeeper --cov-report=html

# Type checking
uv run mypy src/

# Linting
uv run ruff check src/

# Format code
uv run ruff format src/
```

## Migration from pdm

This project was migrated from pdm to uv for better performance and compatibility:

- Removed pdm.lock, .venv, and pdm-specific files
- Updated build backend from pdm to hatchling
- Updated Python version to 3.13
- All commands now use `uv` instead of `pdm`

## License

AGPL-3.0-or-later

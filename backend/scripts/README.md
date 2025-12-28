# Data Migration Scripts

This directory contains scripts to migrate data from MongoDB to PostgreSQL.

## Prerequisites

1. MongoDB must be running and accessible (read-only)
2. PostgreSQL must be running with schema created (`make migrate-up`)
3. Copy `.env.migrate.example` to `.env` and configure

## Usage

```bash
# 1. Ensure PostgreSQL schema is ready
cd ..
make migrate-up

# 2. Configure environment
cp .env.migrate.example .env
# Edit .env with your database credentials

# 3. Run migration
go run scripts/migrate.go
```

## Migration Process

The migration script:

1. **Connects** to MongoDB (read-only) and PostgreSQL
2. **Validates** PostgreSQL schema exists
3. **Asks** for confirmation before starting
4. **Migrates** in order:
   - `wikis` table (primary data)
   - `wiki_stats` table (depends on wikis)
   - `wiki_archives` table (depends on wikis)

## Features

- **Batch inserts**: 100 records per batch for performance
- **Progress tracking**: Shows progress during migration
- **Error handling**: Skips invalid records, logs errors
- **Idempotent**: Checks existing data, skips if already migrated
- **Safe**: MongoDB is read-only, no modifications

## Data Mapping

### MongoDB → PostgreSQL

| MongoDB Type | PostgreSQL Type | Notes |
|-------------|----------------|-------|
| ObjectId (`_id`) | UUID | Converted to UUID v4 |
| ISODate | TIMESTAMP | Direct mapping |
| bool | BOOLEAN | Direct mapping |
| str | VARCHAR/TEXT | Direct mapping |
| int | INTEGER | Direct mapping |

### UUID Conversion

MongoDB `ObjectId` is converted to UUID:
```go
wikiID, err := uuid.Parse(mongoWiki.ID.Hex())
```

## Performance

- **Batch size**: 100 records
- **Expected time**: ~5-10 minutes for 80k wikis
- **Memory usage**: ~50MB for batches

## Troubleshooting

### "PostgreSQL schema not found"
Run: `make migrate-up`

### "Failed to connect to MongoDB"
Check MongoDB is running: `docker-compose ps mongodb`

### "Failed to connect to PostgreSQL"
Check PostgreSQL is running: `docker-compose ps postgres`

### "Skipped records"
Check logs for specific errors. Common issues:
- Invalid UUID format
- Missing required fields
- Foreign key violations

## Safety

- **MongoDB**: READ ONLY access, no deletions or modifications
- **PostgreSQL**: Uses transactions for batch inserts
- **Rollback**: If migration fails, PostgreSQL remains unchanged (within batch)

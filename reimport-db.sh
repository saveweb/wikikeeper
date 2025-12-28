#!/bin/bash
set -e

echo "=== WikiKeeper Database Re-import Script ==="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if backup file exists
if [ ! -f "wikikeeper_backup.sql" ]; then
    echo -e "${RED}Error: wikikeeper_backup.sql not found!${NC}"
    echo "Please place the backup file in the current directory."
    exit 1
fi

echo -e "${YELLOW}This will:${NC}"
echo "  1. Stop and remove existing PostgreSQL container"
echo "  2. Delete all data in ./data directory"
echo "  3. Start fresh PostgreSQL container"
echo "  4. Import wikikeeper_backup.sql"
echo ""

# Confirm
read -p "Continue? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

# Stop and remove container
echo -e "${YELLOW}Step 1: Stopping PostgreSQL container...${NC}"
docker compose stop postgres 2>/dev/null || true
docker compose rm -f postgres 2>/dev/null || true

# Clear data directory
echo -e "${YELLOW}Step 2: Clearing data directory...${NC}"
rm -rf data/*
echo "Data directory cleared."

# Start PostgreSQL
echo -e "${YELLOW}Step 3: Starting PostgreSQL container...${NC}"
docker compose up -d postgres

# Wait for PostgreSQL to be ready
echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    if docker exec wikikeeper-postgres pg_isready -U wikikeeper > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    attempt=$((attempt + 1))
    echo "Waiting... ($attempt/$max_attempts)"
    sleep 2
done

if [ $attempt -eq $max_attempts ]; then
    echo -e "${RED}Error: PostgreSQL did not become ready in time${NC}"
    exit 1
fi

# Import backup
echo -e "${YELLOW}Step 4: Importing wikikeeper_backup.sql...${NC}"
echo "This may take several minutes for large backups..."

# Get file size for progress
backup_size=$(stat -f%z "wikikeeper_backup.sql" 2>/dev/null || stat -c%s "wikikeeper_backup.sql" 2>/dev/null)
backup_size_mb=$((backup_size / 1024 / 1024))
echo "Backup size: ${backup_size_mb}MB"

# Import the data
if docker exec -i wikikeeper-postgres psql -U wikikeeper -d wikikeeper < wikikeeper_backup.sql; then
    echo -e "${GREEN}Import completed successfully!${NC}"
else
    echo -e "${RED}Import failed!${NC}"
    exit 1
fi

# Verify
echo ""
echo -e "${YELLOW}Verifying import...${NC}"
wikis_count=$(docker exec wikikeeper-postgres psql -U wikikeeper -d wikikeeper -t -c "SELECT COUNT(*) FROM wikis;" 2>/dev/null | xargs || echo "0")
stats_count=$(docker exec wikikeeper-postgres psql -U wikikeeper -d wikikeeper -t -c "SELECT COUNT(*) FROM wiki_stats;" 2>/dev/null | xargs || echo "0")
archives_count=$(docker exec wikikeeper-postgres psql -U wikikeeper -d wikikeeper -t -c "SELECT COUNT(*) FROM wiki_archives;" 2>/dev/null | xargs || echo "0")

echo -e "${GREEN}Database statistics:${NC}"
echo "  Wikis: $wikis_count"
echo "  Wiki stats: $stats_count"
echo "  Wiki archives: $archives_count"

echo ""
echo -e "${GREEN}=== Database re-import complete ===${NC}"
echo ""
echo "You can now start the backend:"
echo "  docker compose up -d backend"

#!/bin/bash
set -eo pipefail

# dbackup E2E Verification Script
# This script verifies dbackup against Postgres, MySQL, and SQLite.

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Starting dbackup E2E Verification ===${NC}"

# 1. Build dbackup
echo -e "${BLUE}[1/6] Building dbackup...${NC}"
go build -o ./bin/dbackup main.go
DBACKUP="./bin/dbackup"

# Setup temporary directories
TEST_DIR=$(mktemp -d -t dbackup-e2e-XXXXXX)
BACKUP_DIR="$TEST_DIR/backups"
mkdir -p "$BACKUP_DIR"

cleanup() {
    echo -e "${BLUE}=== Cleaning up ===${NC}"
    docker stop dbackup-e2e-postgres dbackup-e2e-mysql >/dev/null 2>&1 || true
    docker rm dbackup-e2e-postgres dbackup-e2e-mysql >/dev/null 2>&1 || true
    
    # Use docker to clean up files that might be owned by root due to volume mounts
    if [ -d "$TEST_DIR" ]; then
        docker run --rm -v "$TEST_DIR:/tmp/cleanup" alpine sh -c "rm -rf /tmp/cleanup/*" >/dev/null 2>&1 || true
        rm -rf "$TEST_DIR"
    fi
}
trap cleanup EXIT

# 2. Test SQLite
echo -e "${BLUE}[2/6] Testing SQLite...${NC}"
SQLITE_DB="$TEST_DIR/test.db"
sqlite3 "$SQLITE_DB" "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT); INSERT INTO users (name) VALUES ('Alice');"

echo "  Running SQLite Full Backup..."
$DBACKUP backup --db sqlite --db-uri "$SQLITE_DB" --to "$BACKUP_DIR" --compress=false
 
if ls "$BACKUP_DIR"/sqlite-*.sql >/dev/null 2>&1; then
    echo -e "  ${GREEN}PASS: SQLite backup created with engine prefix${NC}"
else
    echo -e "  ${RED}FAIL: SQLite backup missing or wrong prefix${NC}"
    exit 1
fi

# 3. Test Postgres
echo -e "${BLUE}[3/6] Testing Postgres...${NC}"
docker run --name dbackup-e2e-postgres \
    -e POSTGRES_USER=testuser \
    -e POSTGRES_PASSWORD=testpassword \
    -e POSTGRES_DB=testdb \
    -e POSTGRES_HOST_AUTH_METHOD=trust \
    -p 5433:5432 -d postgres:17-alpine \
    -c wal_level=replica -c max_wal_senders=10 -c max_replication_slots=10 -c summarize_wal=on > /dev/null

echo "  Waiting for Postgres to be ready..."
# Wait for pg_isready AND a successful query
max_retries=30
count=0
until docker exec dbackup-e2e-postgres pg_isready -U testuser > /dev/null 2>&1 && \
      docker exec dbackup-e2e-postgres psql -U testuser -d testdb -c "select 1" > /dev/null 2>&1; do
    sleep 1
    count=$((count + 1))
    if [ $count -gt $max_retries ]; then
        echo -e "  ${RED}FAIL: Postgres timed out${NC}"
        docker logs dbackup-e2e-postgres
        exit 1
    fi
done

# Initialize data
docker exec dbackup-e2e-postgres psql -U testuser -d testdb -c "CREATE TABLE data (id SERIAL, val TEXT); INSERT INTO data (val) VALUES ('Initial');"

# Allow replication connections in pg_hba.conf
docker exec dbackup-e2e-postgres sh -c 'echo "host replication testuser all trust" >> /var/lib/postgresql/data/pg_hba.conf'
docker exec dbackup-e2e-postgres psql -U testuser -d testdb -c "SELECT pg_reload_conf();"

echo "  Running Postgres Full Backup..."
$DBACKUP backup --db postgres --host localhost --port 5433 --user testuser --password testpassword --dbname testdb --to "$BACKUP_DIR" --compress=false
 
if ls "$BACKUP_DIR"/postgres-*.sql >/dev/null 2>&1; then
    echo -e "  ${GREEN}PASS: Postgres backup created with engine prefix${NC}"
else
    echo -e "  ${RED}FAIL: Postgres backup missing or wrong prefix${NC}"
    exit 1
fi

# 4. Test MySQL
echo -e "${BLUE}[4/6] Testing MySQL...${NC}"
docker run --name dbackup-e2e-mysql \
  -v "$TEST_DIR/mysql-data:/var/lib/mysql" \
  -e MARIADB_USER=testuser \
  -e MARIADB_PASSWORD=testpassword \
  -e MARIADB_DATABASE=testdb \
  -e MARIADB_ROOT_PASSWORD=rootpass \
  -p 3307:3306 -d mariadb:10.11 --log-bin --binlog-format=ROW --server-id=1 > /dev/null

echo "  Waiting for MySQL/MariaDB to be ready..."
max_retries=30
count=0
until docker exec dbackup-e2e-mysql mariadb-admin ping -u testuser -ptestpassword > /dev/null 2>&1 && \
      docker exec dbackup-e2e-mysql mariadb -u testuser -ptestpassword -e "select 1" > /dev/null 2>&1; do
    sleep 1
    count=$((count + 1))
    if [ $count -gt $max_retries ]; then
        echo -e "  ${RED}FAIL: MySQL timed out${NC}"
        docker logs dbackup-e2e-mysql
        exit 1
    fi
done

# Initialize data and permissions
docker exec dbackup-e2e-mysql mariadb -u root -prootpass -e "GRANT REPLICATION CLIENT, REPLICATION SLAVE ON *.* TO 'testuser'@'%'; FLUSH PRIVILEGES;"
docker exec dbackup-e2e-mysql mariadb -u testuser -ptestpassword testdb -e "CREATE TABLE data (id INT AUTO_INCREMENT PRIMARY KEY, val VARCHAR(255)); INSERT INTO data (val) VALUES ('Initial');"

echo "  Running MySQL Full Backup..."
$DBACKUP backup --db mysql --host localhost --port 3307 --user testuser --password testpassword --dbname testdb --to "$BACKUP_DIR" --compress=false
 
if ls "$BACKUP_DIR"/mysql-*.sql >/dev/null 2>&1; then
     echo -e "  ${GREEN}PASS: MySQL backup created with engine prefix${NC}"
else
    echo -e "  ${RED}FAIL: MySQL backup missing or wrong prefix${NC}"
    exit 1
fi

# 5. Test Restore (SQLite example)
echo -e "${BLUE}[5/6] Testing Restore (SQLite)...${NC}"
RESTORE_DB="$TEST_DIR/restore.db"
BACKUP_FILE=$(ls "$BACKUP_DIR"/sqlite-*.sql | head -n 1)
if $DBACKUP restore --db sqlite --db-uri "$RESTORE_DB" --name "$BACKUP_FILE" --compress=false > /dev/null; then
    COUNT=$(sqlite3 "$RESTORE_DB" "SELECT COUNT(*) FROM users;")
    if [ "$COUNT" -eq 1 ]; then
        echo -e "  ${GREEN}PASS: SQLite Restore verified${NC}"
    else
        echo -e "  ${RED}FAIL: SQLite Restore data mismatch${NC}"
        exit 1
    fi
else
    echo -e "  ${RED}FAIL: SQLite Restore command failed${NC}"
    exit 1
fi

# 6. Edge Cases
echo -e "${BLUE}[6/6] Testing Edge Cases...${NC}"

echo "  Testing Connection Failure (Invalid Host)..."
if $DBACKUP backup --db postgres --host non-existent-host --user u --password p --dbname d --to "$BACKUP_DIR" 2>&1 | grep -qiE "error|failed|lookup"; then
    echo -e "  ${GREEN}PASS: Connection failure caught${NC}"
else
    echo -e "  ${RED}FAIL: Connection failure not reported correctly${NC}"
    exit 1
fi

echo "  Testing Authentication Failure (Wrong Password)..."
if $DBACKUP backup --db mysql --host localhost --port 3307 --user testuser --password WRONGPASS --dbname testdb --to "$BACKUP_DIR" 2>&1 | grep -qiE "error|failed|denied"; then
    echo -e "  ${GREEN}PASS: Auth failure caught${NC}"
else
    echo -e "  ${RED}FAIL: Auth failure not reported correctly${NC}"
    exit 1
fi

echo -e "${GREEN}=== All E2E Tests Passed! ===${NC}"

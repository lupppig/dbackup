#!/bin/bash

# dummy_backup.sh - A simple script to demonstrate dbackup with dummy data
set -e

DB_FILE="dummy.db"
BACKUP_DIR="./dummy_backups"

echo "=== 1. Building dbackup ==="
make build
# Ensure bin is in path for this script
export PATH="$PWD/bin:$PATH"

echo "=== 2. Creating Dummy SQLite Database ==="
rm -f "$DB_FILE"
sqlite3 "$DB_FILE" <<EOF
CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, email TEXT);
INSERT INTO users (name, email) VALUES ('Alice Smith', 'alice@example.com');
INSERT INTO users (name, email) VALUES ('Bob Jones', 'bob@example.com');
INSERT INTO users (name, email) VALUES ('Charlie Brown', 'charlie@example.com');
EOF

echo "Dummy database created at $DB_FILE with 3 records."

echo "=== 3. Backing Up with dbackup ==="
# Backup the dummy sqlite database to a local directory
dbackup backup sqlite --db "$DB_FILE" --to "local://$BACKUP_DIR"

echo "=== 4. Verifying Backups ==="
# List the backups
dbackup backups --to "local://$BACKUP_DIR" --db "$DB_FILE"

echo "=== Done! ==="
echo "You can find your dummy database at: $DB_FILE"
echo "You can find your backups at: $BACKUP_DIR"

#!/bin/bash
set -e
 
# 1. Setup
TEST_DIR=$(mktemp -d -t dbackup-cas-XXXXXX)
DB_PATH="$TEST_DIR/test.db"
STORAGE_DIR="$TEST_DIR/storage"
mkdir -p "$STORAGE_DIR"
 
echo "Creating test SQLite database..."
sqlite3 "$DB_PATH" "CREATE TABLE items (id INTEGER PRIMARY KEY, val TEXT); INSERT INTO items (val) VALUES ('Initial data');"
 
# 2. First Backup
echo "Running First Backup with --dedupe..."
./bin/dbackup backup --db sqlite --db-uri "$DB_PATH" --to "$STORAGE_DIR" --dedupe --compress=false
 
FIRST_CHUNKS=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "First backup chunks: $FIRST_CHUNKS"
 
# 3. Second Backup (Identical data)
echo "Running Second Backup with --dedupe..."
./bin/dbackup backup --db sqlite --db-uri "$DB_PATH" --to "$STORAGE_DIR" --dedupe --compress=false
 
SECOND_CHUNKS=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "Second backup chunks: $SECOND_CHUNKS"
 
if [ "$FIRST_CHUNKS" -eq "$SECOND_CHUNKS" ]; then
    echo "SUCCESS: No new chunks created for identical data. Idempotency verified."
else
    echo "FAILURE: Redundant chunks created for identical data."
    exit 1
fi
 
# 4. Third Backup (Modified data)
echo "Modifying data and running Third Backup..."
sqlite3 "$DB_PATH" "INSERT INTO items (val) VALUES ('New data');"
./bin/dbackup backup --db sqlite --db-uri "$DB_PATH" --to "$STORAGE_DIR" --dedupe --compress=false
 
THIRD_CHUNKS=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "Third backup chunks: $THIRD_CHUNKS"
 
if [ "$THIRD_CHUNKS" -gt "$SECOND_CHUNKS" ]; then
    echo "SUCCESS: New chunks created for modified data."
else
    echo "FAILURE: No new chunks created for modified data."
    exit 1
fi

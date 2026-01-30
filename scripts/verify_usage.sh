#!/bin/bash
set -e
 
TEST_DIR=$(mktemp -d -t dbackup-usage-XXXXXX)
DB_PATH="$TEST_DIR/test.db"
STORAGE_DIR="$TEST_DIR/storage"
mkdir -p "$STORAGE_DIR"
 
# 1. Create a "large" database (~2MB)
echo "Creating 2MB SQLite database..."
sqlite3 "$DB_PATH" "CREATE TABLE data (val TEXT);"
for i in {1..1000}; do
    sqlite3 "$DB_PATH" "INSERT INTO data (val) VALUES ('$(printf 'Data block %04d ' $i)$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | head -c 2000)');"
done
 
# 2. First Backup
echo "Running First Backup..."
./bin/dbackup backup --db sqlite --db-uri "$DB_PATH" --to "$STORAGE_DIR" --compress=false
 
SIZE1=$(du -sb "$STORAGE_DIR/chunks" | cut -f1)
CHUNKS1=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "Storage size after first backup: $SIZE1 bytes ($CHUNKS1 chunks)"
 
# 3. Second Backup (Slight modification)
echo "Modifying data and running Second Backup..."
sqlite3 "$DB_PATH" "INSERT INTO data (val) VALUES ('Small change at the end of the file to test dedupe');"
./bin/dbackup backup --db sqlite --db-uri "$DB_PATH" --to "$STORAGE_DIR" --compress=false
 
SIZE2=$(du -sb "$STORAGE_DIR/chunks" | cut -f1)
CHUNKS2=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "Storage size after second backup: $SIZE2 bytes ($CHUNKS2 chunks)"
 
# The data grew by ~2KB. One chunk should be added (~64KB-512KB).
# The total size should be MUCH less than SIZE1 * 2.
 
GROWTH=$((SIZE2 - SIZE1))
echo "Actual growth: $GROWTH bytes"
 
if [ "$GROWTH" -lt 524288 ]; then # Less than 512KB (the max chunk size)
    echo "SUCCESS: Deduplication is highly effective! (Growth < 1 chunk)"
else
    echo "FAILURE: Storage growth is too high ($GROWTH bytes)."
    exit 1
fi

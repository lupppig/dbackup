#!/bin/bash
set -e
 
TEST_DIR=$(mktemp -d -t dbackup-cdc-XXXXXX)
STORAGE_DIR="$TEST_DIR/storage"
mkdir -p "$STORAGE_DIR"
 
# Simulate a Postgres dump with a timestamp
create_dump() {
    local ts=$1
    echo "-- Dumped on $ts"
    echo "CREATE TABLE users (id SERIAL, name TEXT);"
    for i in {1..1000}; do
        echo "INSERT INTO users (name) VALUES ('User $i');"
    done
}
 
echo "Generating first dump..."
create_dump "2026-01-30 15:00:00" > "$TEST_DIR/dump1.sql"
 
echo "Running First Backup with --dedupe..."
./bin/dbackup backup --db postgres --db-uri "$TEST_DIR/dump1.sql" --to "$STORAGE_DIR" --dedupe --compress=false
 
FIRST_CHUNKS=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "First backup chunks: $FIRST_CHUNKS"
 
echo "Generating second dump with DIFFERENT timestamp..."
create_dump "2026-01-30 15:10:00" > "$TEST_DIR/dump2.sql"
 
echo "Running Second Backup with --dedupe..."
./bin/dbackup backup --db postgres --db-uri "$TEST_DIR/dump2.sql" --to "$STORAGE_DIR" --dedupe --compress=false
 
TOTAL_CHUNKS=$(ls "$STORAGE_DIR/chunks" | wc -l)
echo "Total chunks after second backup: $TOTAL_CHUNKS"
 
NEW_CHUNKS=$((TOTAL_CHUNKS - FIRST_CHUNKS))
echo "New chunks created: $NEW_CHUNKS"
 
if [ "$NEW_CHUNKS" -lt "$FIRST_CHUNKS" ]; then
    echo "SUCCESS: CDC isolated the timestamp change. Most chunks were deduplicated."
else
    echo "FAILURE: CDC failed to deduplicate data after a header change."
    exit 1
fi

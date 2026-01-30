#!/bin/bash
set -e
 
TEST_DIR=$(mktemp -d -t dbackup-multi-XXXXXX)
DB1="$TEST_DIR/db1.sqlite"
DB2="$TEST_DIR/db2.sqlite"
STORAGE="$TEST_DIR/storage"
mkdir -p "$STORAGE"
 
# 1. Create DB1
sqlite3 "$DB1" "CREATE TABLE t1 (id INTEGER PRIMARY KEY, Val TEXT); INSERT INTO t1 (Val) VALUES ('DB1 Data');"
# 2. Create DB2
sqlite3 "$DB2" "CREATE TABLE t2 (id INTEGER PRIMARY KEY, Val TEXT); INSERT INTO t2 (Val) VALUES ('DB2 Data');"
 
# 3. Run Multi-Backup
echo "Running multi-backup for $DB1 and $DB2..."
./bin/dbackup backup --to "$STORAGE" "sqlite://$DB1" "sqlite://$DB2"
 
# 4. Verify output
echo "Checking storage contents..."
ls -R "$STORAGE"
 
MANIFEST_COUNT=$(find "$STORAGE" -name "*.manifest" | wc -l)
if [ "$MANIFEST_COUNT" -eq 2 ]; then
    echo "SUCCESS: Found 2 manifest files."
else
    echo "FAILURE: Expected 2 manifest files, found $MANIFEST_COUNT."
    exit 1
fi
 
# 5. Try Multi-Restore (Simulated)
echo "Running multi-restore..."
M1=$(ls "$STORAGE/backups" | grep "sqlite" | head -n 1)
M2=$(ls "$STORAGE/backups" | grep "sqlite" | tail -n 1)
 
RESTORE1="$TEST_DIR/restored1.sqlite"
RESTORE2="$TEST_DIR/restored2.sqlite"
 
# Using manifest:uri format
./bin/dbackup restore --to "$STORAGE" "backups/$M1:sqlite://$RESTORE1" "backups/$M2:sqlite://$RESTORE2"
 
if [ -f "$RESTORE1" ] && [ -f "$RESTORE2" ]; then
    echo "SUCCESS: Both databases restored."
else
    echo "FAILURE: Restoration failed."
    exit 1
fi

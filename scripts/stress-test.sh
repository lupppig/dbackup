#!/bin/bash
# =============================================================================
# dbackup Stress Test Suite
# =============================================================================
# Tests backup performance across:
#   - Databases: MySQL (Docker), PostgreSQL (Docker), SQLite
#   - Data sizes: 50K, 50M, 100M rows
#   - Targets: Local, SSH (2 VMs), Docker container
#   - Operations: backup, restore, parallel backup/restore, schedule
# =============================================================================

set -e

# -----------------------------------------------------------------------------
# Configuration (Edit these variables)
# -----------------------------------------------------------------------------
VM1_SSH="centosuser@192.168.1.90"
VM2_SSH="ubuntu@192.168.1.159"
VM1_PATH="backups"
VM2_PATH="backups"

DOCKER_CONTAINER="dbackup-storage"
DOCKER_PATH="/backups"

# Data sizes to test (rows per table)
DATA_SIZES=(50000 1000000 50000000 100000000)

# Tables per database
NUM_TABLES=5

# Parallelism levels to test
PARALLELISM_LEVELS=(1 4 8)

# Encryption passphrase for tests
ENCRYPTION_KEY="stress-test-key-2026"

# Docker container settings
MYSQL_CONTAINER="dbackup-mysql"
POSTGRES_CONTAINER="dbackup-postgres"
MYSQL_ROOT_PASSWORD="rootpassword"
MYSQL_PORT=3306
POSTGRES_PASSWORD="postgrespassword"
POSTGRES_PORT=5433  # Using 5433 to avoid conflict with other postgres instances

# Paths
DBACKUP_BIN="./bin/dbackup"
TEST_DIR="./stress-test-data"
RESULTS_FILE="./stress-test-results.log"
SQLITE_DB="$TEST_DIR/stress-test.db"  # Full path for manifest compatibility

# -----------------------------------------------------------------------------
# Utility Functions
# -----------------------------------------------------------------------------
VERBOSE=${VERBOSE:-false}  # Set to true to show command output
QUIET=${QUIET:-true}        # Set to false to show setup/progress messages

log() {
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    local msg="[$timestamp] $1"
    echo "$msg" >> "$RESULTS_FILE"
    if [ "$QUIET" != "true" ]; then
        echo "$msg"
    fi
}

# Always show progress for long-running operations (data generation)
log_progress() {
    local msg="$1"
    echo -ne "\r\033[K$msg" >&2
}

log_progress_done() {
    echo "" >&2
}

log_result() {
    local operation="$1"
    local db_type="$2"
    local data_size="$3"
    local target="$4"
    local duration="$5"
    local status="$6"
    
    # Color codes
    local GREEN='\033[0;32m'
    local RED='\033[0;31m'
    local NC='\033[0m' # No Color
    
    local status_colored="$status"
    if [ "$status" = "OK" ]; then
        status_colored="${GREEN}✓ OK${NC}"
    else
        status_colored="${RED}✗ FAIL${NC}"
    fi
    
    # Print to stdout with colors
    printf "| %-20s | %-10s | %12s | %-15s | %10s | %b |\n" \
        "$operation" "$db_type" "$data_size" "$target" "${duration}s" "$status_colored"
    
    # Print to file without colors
    printf "| %-20s | %-10s | %12s | %-15s | %10s | %-7s |\n" \
        "$operation" "$db_type" "$data_size" "$target" "${duration}s" "$status" >> "$RESULTS_FILE"
}

format_number() {
    printf "%'d" "$1"
}

print_table_header() {
    echo ""
    echo "┌──────────────────────┬────────────┬──────────────┬─────────────────┬────────────┬─────────┐"
    printf "│ %-20s │ %-10s │ %12s │ %-15s │ %10s │ %-7s │\n" \
        "Operation" "Database" "Rows" "Target" "Duration" "Status"
    echo "├──────────────────────┼────────────┼──────────────┼─────────────────┼────────────┼─────────┤"
    
    # Also write to file
    echo "" >> "$RESULTS_FILE"
    printf "| %-20s | %-10s | %12s | %-15s | %10s | %-7s |\n" \
        "Operation" "Database" "Rows" "Target" "Duration" "Status" >> "$RESULTS_FILE"
    echo "|----------------------|------------|--------------|-----------------|------------|---------|" >> "$RESULTS_FILE"
}

print_table_footer() {
    echo "└──────────────────────┴────────────┴──────────────┴─────────────────┴────────────┴─────────┘"
    echo ""
}

time_command() {
    local start_time=$(date +%s.%N)
    if [ "$VERBOSE" = "true" ]; then
        # Redirect command output to stderr so it doesn't mix with timing result
        "$@" >&2 2>&1
    else
        "$@" >/dev/null 2>&1
    fi
    local exit_code=$?
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    # Only return timing data to stdout
    printf "%.2f %d" "$duration" "$exit_code"
}

cleanup_vms() {
    log "Cleaning up remote VMs..."
    ssh "$VM1_SSH" "rm -rf $VM1_PATH/*" || true
    ssh "$VM2_SSH" "rm -rf $VM2_PATH/*" || true
}

cleanup() {
    log "Cleaning up..."
    cleanup_vms
    docker rm -f "$MYSQL_CONTAINER" "$POSTGRES_CONTAINER" "$DOCKER_CONTAINER" 2>/dev/null || true
    rm -rf "$TEST_DIR" 2>/dev/null || true
}

ensure_containers_running() {
    # Restart containers if they stopped
    docker start "$MYSQL_CONTAINER" 2>/dev/null || true
    docker start "$POSTGRES_CONTAINER" 2>/dev/null || true
    docker start "$DOCKER_CONTAINER" 2>/dev/null || true
    sleep 2
}

# -----------------------------------------------------------------------------
# Docker Setup
# -----------------------------------------------------------------------------
setup_docker_containers() {
    log "Setting up Docker containers..."
    
    # MySQL
    log "Starting MySQL container..."
    docker run -d --name "$MYSQL_CONTAINER" \
        -e MYSQL_ROOT_PASSWORD="$MYSQL_ROOT_PASSWORD" \
        -e MYSQL_DATABASE=stresstest \
        -p 3306:3306 \
        mysql:8.0 2>/dev/null || log "MySQL container already exists"
    
    # PostgreSQL
    log "Starting PostgreSQL container..."
    docker run -d --name "$POSTGRES_CONTAINER" \
        -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        -e POSTGRES_DB=stresstest \
        -p $POSTGRES_PORT:5432 \
        postgres:15 2>/dev/null || log "PostgreSQL container already exists"
    
    # Storage container for Docker backup target
    log "Starting storage container..."
    docker run -d --name "$DOCKER_CONTAINER" \
        -v /backups \
        alpine:latest tail -f /dev/null 2>/dev/null || log "Storage container already exists"
    
    # Ensure containers are actually running (restart if stopped)
    docker start "$MYSQL_CONTAINER" 2>/dev/null || true
    docker start "$POSTGRES_CONTAINER" 2>/dev/null || true
    docker start "$DOCKER_CONTAINER" 2>/dev/null || true
    
    # Wait for databases to be ready
    log "Waiting for databases to initialize..."
    sleep 20
    
    # Wait for MySQL
    log "Waiting for MySQL..."
    for i in {1..60}; do
        if docker exec "$MYSQL_CONTAINER" mysqladmin ping -h localhost -u root -p"$MYSQL_ROOT_PASSWORD" --silent 2>/dev/null; then
            log "MySQL is ready"
            break
        fi
        sleep 2
    done
    
    # Wait for PostgreSQL
    log "Waiting for PostgreSQL..."
    for i in {1..60}; do
        if docker exec "$POSTGRES_CONTAINER" pg_isready -U postgres 2>/dev/null; then
            log "PostgreSQL is ready"
            break
        fi
        sleep 2
    done
}

# -----------------------------------------------------------------------------
# Data Generation
# -----------------------------------------------------------------------------
generate_sqlite_data() {
    local num_rows=$1
    local db_path=$2
    
    log "Generating SQLite data: $(format_number $num_rows) rows across $NUM_TABLES tables..."
    
    rm -f "$db_path"
    
    for t in $(seq 1 $NUM_TABLES); do
        sqlite3 "$db_path" "CREATE TABLE IF NOT EXISTS test_table_$t (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            uuid TEXT NOT NULL,
            name TEXT NOT NULL,
            email TEXT NOT NULL,
            data BLOB,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );"
        
        local rows_per_table=$((num_rows / NUM_TABLES))
        local batch_size=10000
        local batches=$((rows_per_table / batch_size))
        
        log "  Table $t: Inserting $(format_number $rows_per_table) rows..."
        
        for b in $(seq 1 $batches); do
            sqlite3 "$db_path" "
                INSERT INTO test_table_$t (uuid, name, email, data)
                SELECT 
                    lower(hex(randomblob(16))),
                    'User_' || abs(random() % 1000000),
                    'user' || abs(random() % 1000000) || '@example.com',
                    randomblob(100)
                FROM (
                    WITH RECURSIVE cnt(x) AS (
                        SELECT 1 UNION ALL SELECT x+1 FROM cnt WHERE x < $batch_size
                    ) SELECT x FROM cnt
                );
            " 2>/dev/null || true
            
            if [ $((b % 10)) -eq 0 ]; then
                log_progress "    Progress: $((b * batch_size)) / $rows_per_table"
            fi
        done
        log_progress_done
    done
    
    local size=$(du -h "$db_path" | cut -f1)
    log "SQLite database created: $size"
}

generate_mysql_data() {
    local num_rows=$1
    
    log "Generating MySQL data: $(format_number $num_rows) rows across $NUM_TABLES tables..."
    
    for t in $(seq 1 $NUM_TABLES); do
        docker exec -i "$MYSQL_CONTAINER" mysql -u root -p"$MYSQL_ROOT_PASSWORD" stresstest <<EOF
CREATE TABLE IF NOT EXISTS test_table_$t (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    data BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
TRUNCATE TABLE test_table_$t;
EOF
        
        local rows_per_table=$((num_rows / NUM_TABLES))
        local batch_size=50000
        local batches=$((rows_per_table / batch_size))
        
        log "  Table $t: Inserting $(format_number $rows_per_table) rows..."
        
        for b in $(seq 1 $batches); do
            # Use a more reliable way to generate batch data in MySQL
            docker exec -i "$MYSQL_CONTAINER" mysql -u root -p"$MYSQL_ROOT_PASSWORD" stresstest <<EOF
INSERT INTO test_table_$t (uuid, name, email, data)
SELECT 
    UUID(),
    CONCAT('User_', FLOOR(RAND() * 1000000)),
    CONCAT('user', FLOOR(RAND() * 1000000), '@example.com'),
    RANDOM_BYTES(96)
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 + d.N * 1000 + e.N * 10000 AS n
    FROM (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) c
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) d
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) e
    LIMIT $batch_size
) numbers;
EOF
            if [ $((b % 1)) -eq 0 ]; then
                log_progress "    Progress: $((b * batch_size)) / $rows_per_table"
            fi
        done
        log_progress_done
    done
    
    log "MySQL data generation complete"
}

generate_postgres_data() {
    local num_rows=$1
    
    log "Generating PostgreSQL data: $(format_number $num_rows) rows across $NUM_TABLES tables..."
    
    for t in $(seq 1 $NUM_TABLES); do
        docker exec -i "$POSTGRES_CONTAINER" psql -U postgres -d stresstest <<EOF
CREATE TABLE IF NOT EXISTS test_table_$t (
    id BIGSERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    data BYTEA,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
TRUNCATE TABLE test_table_$t;
EOF
        
        local rows_per_table=$((num_rows / NUM_TABLES))
        log "  Table $t: Inserting $(format_number $rows_per_table) rows..."
        
        docker exec -i "$POSTGRES_CONTAINER" psql -U postgres -d stresstest <<EOF
INSERT INTO test_table_$t (uuid, name, email, data)
SELECT 
    gen_random_uuid(),
    'User_' || (random() * 1000000)::int,
    'user' || (random() * 1000000)::int || '@example.com',
    decode(repeat(md5(random()::text), 3), 'hex')
FROM generate_series(1, $rows_per_table);
EOF
    done
    
    log "PostgreSQL data generation complete"
}

# -----------------------------------------------------------------------------
# Test Functions
# -----------------------------------------------------------------------------
test_backup() {
    local db_type=$1
    local target=$2
    local target_name=$3
    local data_size=$4
    local encrypt=$5
    
    local cmd_args="--confirm-restore"
    local target_uri=""
    
    case $target_name in
        "local")
            target_uri="$TEST_DIR/backups-$db_type-$data_size"
            mkdir -p "$target_uri"
            ;;
        "vm1")
            target_uri="sftp://$VM1_SSH/$VM1_PATH"
            ;;
        "vm2")
            target_uri="sftp://$VM2_SSH/$VM2_PATH"
            ;;
        "docker")
            target_uri="docker:$DOCKER_CONTAINER:$DOCKER_PATH"
            ;;
    esac
    
    local encrypt_args=""
    if [ "$encrypt" = "true" ]; then
        encrypt_args="--encrypt --encryption-passphrase $ENCRYPTION_KEY"
    fi
    
    case $db_type in
        "sqlite")
            local result=$(time_command $DBACKUP_BIN backup sqlite --db "$SQLITE_DB" --to "$target_uri" $encrypt_args)
            ;;
        "mysql")
            local result=$(time_command $DBACKUP_BIN backup mysql --db stresstest --host localhost --port 3306 --user root --password "$MYSQL_ROOT_PASSWORD" --to "$target_uri" $encrypt_args)
            ;;
        "postgres")
            local result=$(time_command $DBACKUP_BIN backup postgres --db stresstest --host localhost --port $POSTGRES_PORT --user postgres --password "$POSTGRES_PASSWORD" --to "$target_uri" $encrypt_args)
            ;;
    esac
    
    local duration=$(echo "$result" | awk '{print $1}')
    local exit_code=$(echo "$result" | awk '{print $2}')
    local status="OK"
    [ "$exit_code" != "0" ] && status="FAIL"
    
    local op_name="backup"
    [ "$encrypt" = "true" ] && op_name="backup+encrypt"
    
    log_result "$op_name" "$db_type" "$data_size" "$target_name" "$duration" "$status"
}

test_restore() {
    local db_type=$1
    local target=$2
    local target_name=$3
    local data_size=$4
    
    local target_uri=""
    
    case $target_name in
        "local")
            target_uri="$TEST_DIR/backups-$db_type-$data_size"
            ;;
        "vm1")
            target_uri="sftp://$VM1_SSH/$VM1_PATH"
            ;;
        "vm2")
            target_uri="sftp://$VM2_SSH/$VM2_PATH"
            ;;
        "docker")
            target_uri="docker:$DOCKER_CONTAINER:$DOCKER_PATH"
            ;;
    esac
    
    # Auto-restore uses manifest DBName, so we don't specify --db for SQLite
    # We need --dedupe since backups use deduplication
    # Add encryption passphrase in case some backups are encrypted
    case $db_type in
        "sqlite")
            # Use auto-restore mode (no --db flag, it uses manifest's dbname)
            local result=$(time_command $DBACKUP_BIN restore sqlite --from "$target_uri" --dedupe --encryption-passphrase "$ENCRYPTION_KEY" --confirm-restore)
            ;;
        "mysql")
            local result=$(time_command $DBACKUP_BIN restore mysql --db stresstest_restored --host localhost --port $MYSQL_PORT --user root --password "$MYSQL_ROOT_PASSWORD" --from "$target_uri" --dedupe --encryption-passphrase "$ENCRYPTION_KEY" --confirm-restore)
            ;;
        "postgres")
            local result=$(time_command $DBACKUP_BIN restore postgres --db stresstest_restored --host localhost --port $POSTGRES_PORT --user postgres --password "$POSTGRES_PASSWORD" --from "$target_uri" --dedupe --encryption-passphrase "$ENCRYPTION_KEY" --confirm-restore)
            ;;
    esac
    
    local duration=$(echo "$result" | awk '{print $1}')
    local exit_code=$(echo "$result" | awk '{print $2}')
    local status="OK"
    [ "$exit_code" != "0" ] && status="FAIL"
    
    log_result "restore" "$db_type" "$data_size" "$target_name" "$duration" "$status"
}

test_parallel_backup() {
    local db_type=$1
    local parallelism=$2
    local data_size=$3
    
    local target_uri="$TEST_DIR/parallel-backups-$db_type-$data_size"
    mkdir -p "$target_uri"
    
    case $db_type in
        "sqlite")
            local result=$(time_command $DBACKUP_BIN backup sqlite --db "$SQLITE_DB" --to "$target_uri" --parallelism "$parallelism")
            ;;
        "mysql")
            local result=$(time_command $DBACKUP_BIN backup mysql --db stresstest --host localhost --port 3306 --user root --password "$MYSQL_ROOT_PASSWORD" --to "$target_uri" --parallelism "$parallelism")
            ;;
        "postgres")
            local result=$(time_command $DBACKUP_BIN backup postgres --db stresstest --host localhost --port $POSTGRES_PORT --user postgres --password "$POSTGRES_PASSWORD" --to "$target_uri" --parallelism "$parallelism")
            ;;
    esac
    
    local duration=$(echo "$result" | awk '{print $1}')
    local exit_code=$(echo "$result" | awk '{print $2}')
    local status="OK"
    [ "$exit_code" != "0" ] && status="FAIL"
    
    log_result "parallel-backup(p=$parallelism)" "$db_type" "$data_size" "local" "$duration" "$status"
}

test_schedule() {
    local db_type=$1
    local data_size=$2
    
    local target_uri="$TEST_DIR/scheduled-backups-$db_type"
    mkdir -p "$target_uri"
    
    log "Testing scheduler with $db_type..."
    
    # Test 1: Add a scheduled backup task (schedule backup [engine], not --engine flag)
    local add_result=""
    case $db_type in
        "sqlite")
            add_result=$(time_command $DBACKUP_BIN schedule backup sqlite --cron "*/5 * * * *" --db "$SQLITE_DB" --to "$target_uri")
            ;;
        "mysql")
            add_result=$(time_command $DBACKUP_BIN schedule backup mysql --cron "*/5 * * * *" --db stresstest --host localhost --port $MYSQL_PORT --user root --password "$MYSQL_ROOT_PASSWORD" --to "$target_uri")
            ;;
        "postgres")
            add_result=$(time_command $DBACKUP_BIN schedule backup postgres --cron "*/5 * * * *" --db stresstest --host localhost --port $POSTGRES_PORT --user postgres --password "$POSTGRES_PASSWORD" --to "$target_uri")
            ;;
    esac
    
    local add_duration=$(echo "$add_result" | awk '{print $1}')
    local add_exit=$(echo "$add_result" | awk '{print $2}')
    local add_status="OK"
    [ "$add_exit" != "0" ] && add_status="FAIL"
    log_result "schedule-backup" "$db_type" "$data_size" "local" "$add_duration" "$add_status"
    
    # Test 2: List scheduled tasks
    local list_result=$(time_command $DBACKUP_BIN schedule list)
    local list_duration=$(echo "$list_result" | awk '{print $1}')
    local list_exit=$(echo "$list_result" | awk '{print $2}')
    local list_status="OK"
    [ "$list_exit" != "0" ] && list_status="FAIL"
    log_result "schedule-list" "$db_type" "$data_size" "local" "$list_duration" "$list_status"
}

# -----------------------------------------------------------------------------
# Main Test Runner
# -----------------------------------------------------------------------------
run_tests() {
    log "============================================================"
    log "          dbackup Stress Test Suite"
    log "============================================================"
    log ""
    log "Configuration:"
    log "  VM1: $VM1_SSH"
    log "  VM2: $VM2_SSH"
    log "  Data sizes: ${DATA_SIZES[*]}"
    log "  Tables per DB: $NUM_TABLES"
    log "  Parallelism levels: ${PARALLELISM_LEVELS[*]}"
    log ""
    
    # Print table header
    print_table_header
    
    for data_size in "${DATA_SIZES[@]}"; do
        log ""
        log ">>> Testing with $(format_number $data_size) rows <<<"
        log ""
        
        # =====================================================================
        # SQLite Tests
        # =====================================================================
        log "--- SQLite Tests ---"
        generate_sqlite_data "$data_size" "$SQLITE_DB"
        
        # Local backup
        test_backup "sqlite" "local" "local" "$data_size" "false"
        test_backup "sqlite" "local" "local" "$data_size" "true"
        
        # Restore from local
        test_restore "sqlite" "local" "local" "$data_size"
        
        # Parallel backup tests
        for p in "${PARALLELISM_LEVELS[@]}"; do
            test_parallel_backup "sqlite" "$p" "$data_size"
        done
        
        # VM backups (requires SSH key auth)
        test_backup "sqlite" "vm1" "vm1" "$data_size" "false"
        test_backup "sqlite" "vm2" "vm2" "$data_size" "false"
        
        # Docker backup
        test_backup "sqlite" "docker" "docker" "$data_size" "false"
        
        # Schedule test
        test_schedule "sqlite" "$data_size"
        
        # =====================================================================
        # MySQL Tests (only for smaller data sizes due to time)
        # =====================================================================
        if [ "$data_size" -le 50000000 ]; then
            log ""
            log "--- MySQL Tests ---"
            ensure_containers_running
            generate_mysql_data "$data_size"
            
            test_backup "mysql" "local" "local" "$data_size" "false"
            test_backup "mysql" "local" "local" "$data_size" "true"
            test_restore "mysql" "local" "local" "$data_size"
            
            for p in "${PARALLELISM_LEVELS[@]}"; do
                test_parallel_backup "mysql" "$p" "$data_size"
            done
            
            # VM backups
            test_backup "mysql" "vm1" "vm1" "$data_size" "false"
            test_backup "mysql" "vm2" "vm2" "$data_size" "false"
            
            test_schedule "mysql" "$data_size"
        fi
        
        # =====================================================================
        # PostgreSQL Tests
        # =====================================================================
        if [ "$data_size" -le 50000000 ]; then
            log ""
            log "--- PostgreSQL Tests ---"
            ensure_containers_running
            generate_postgres_data "$data_size"
            
            test_backup "postgres" "local" "local" "$data_size" "false"
            test_backup "postgres" "local" "local" "$data_size" "true"
            test_restore "postgres" "local" "local" "$data_size"
            
            for p in "${PARALLELISM_LEVELS[@]}"; do
                test_parallel_backup "postgres" "$p" "$data_size"
            done
            
            # VM backups
            test_backup "postgres" "vm1" "vm1" "$data_size" "false"
            test_backup "postgres" "vm2" "vm2" "$data_size" "false"
            
            test_schedule "postgres" "$data_size"
        fi
    done
    
    print_table_footer
    
    log ""
    log "============================================================"
    log "          Stress Test Complete"
    log "============================================================"
    log "Results saved to: $RESULTS_FILE"
}

# -----------------------------------------------------------------------------
# Entry Point
# -----------------------------------------------------------------------------
main() {
    # Build dbackup first
    log "Building dbackup..."
    go build -o "$DBACKUP_BIN" . || { log "Failed to build dbackup"; exit 1; }
    
    # Create test directory
    mkdir -p "$TEST_DIR"
    
    # Clear previous results
    > "$RESULTS_FILE"
    
    # Setup
    setup_docker_containers
    
    # Run tests
    run_tests
    
    # Summary
    log ""
    log "Test Summary:"
    grep -E "^\|" "$RESULTS_FILE" | tail -n +3
}

# Trap for cleanup on exit (disabled for debugging)
# trap cleanup EXIT

# Parse arguments
case "${1:-run}" in
    "run")
        main
        ;;
    "run-50k")
        DATA_SIZES=(50000)
        main
        ;;
    "run-1m")
        DATA_SIZES=(1000000)
        main
        ;;
    "run-50m")
        DATA_SIZES=(50000000)
        main
        ;;
    "run-100m")
        DATA_SIZES=(100000000)
        main
        ;;
    "setup")
        setup_docker_containers
        ;;
    "cleanup")
        cleanup
        ;;
    *)
        echo "Usage: $0 [run|run-50k|run-1m|run-50m|run-100m|setup|cleanup]"
        echo ""
        echo "Commands:"
        echo "  run       - Run all tests (50K, 1M, 50M, 100M rows)"
        echo "  run-50k   - Run only 50K row tests"
        echo "  run-1m    - Run only 1M row tests"
        echo "  run-50m   - Run only 50M row tests"
        echo "  run-100m  - Run only 100M row tests"
        echo "  setup     - Setup Docker containers only"
        echo "  cleanup   - Remove Docker containers and test data"
        exit 1
        ;;
esac

#!/bin/bash

# Exit on error
set -e

echo "=== Starting dbackup Simulation Environment ==="

# 1. Start Docker containers
echo "Step 1: Launching Docker containers (Postgres, MySQL, MariaDB, MinIO, SFTP)..."
docker compose up -d

# 2. Give containers time to be ready
echo "Step 2: Waiting for databases to initialize..."
sleep 15

# 3. Seed databases
echo "Step 3: Seeding databases with test data..."
./scripts/seed_dbs.sh

# 4. Launch dbackup dump
echo "Step 4: Launching dbackup dump with minute-based schedules..."
echo "Press Ctrl+C to stop the simulation."
go run main.go dump --config ~/.dbackup/backup.yaml

#!/bin/bash

# Helper function to wait for a container to be running
wait_for_container() {
  local service=$1
  echo "Waiting for $service..."
  for i in {1..30}; do
    if docker compose ps --format json "$service" | grep -q '"State":"running"'; then
      return 0
    fi
    sleep 1
  done
  echo "Error: $service failed to start"
  return 1
}

# Seed Postgres
wait_for_container postgres
echo "Seeding Postgres..."
docker exec -i $(docker compose ps -q postgres) psql -U user -d testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('Postgres User 1'), ('Postgres User 2');
EOF

# Seed MySQL
wait_for_container mysql
echo "Seeding MySQL..."
docker exec -i $(docker compose ps -q mysql) mysql -uuser -ppassword testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));
INSERT INTO users (name) VALUES ('MySQL User 1'), ('MySQL User 2');
EOF

# Seed MariaDB
wait_for_container mariadb
echo "Seeding MariaDB..."
docker exec -i $(docker compose ps -q mariadb) mariadb -uuser -ppassword testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));
INSERT INTO users (name) VALUES ('MariaDB User 1'), ('MariaDB User 2');
EOF

# Seed SQLite
echo "Seeding SQLite..."
sqlite3 test.sqlite <<EOF
CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('SQLite User 1'), ('SQLite User 2');
EOF

# Create MinIO Bucket
wait_for_container minio
echo "Creating MinIO bucket 'backups'..."
docker run --net=host --entrypoint=/bin/sh minio/mc -c "
  mc alias set myminio http://localhost:9000 minioadmin minioadmin;
  mc mb --ignore-existing myminio/backups;
" 2>/dev/null || echo "Bucket creation failed (it might already exist), moving on..."

echo "Done seeding databases and preparing MinIO!"

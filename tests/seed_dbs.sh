#!/bin/bash

# Seed Postgres
echo "Seeding Postgres..."
docker exec -i $(docker compose -f tests/docker-compose.yaml ps -q postgres) psql -U user -d testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('Postgres User 1'), ('Postgres User 2');
EOF

# Seed MySQL
echo "Seeding MySQL..."
docker exec -i $(docker compose -f tests/docker-compose.yaml ps -q mysql) mysql -uuser -ppassword testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));
INSERT INTO users (name) VALUES ('MySQL User 1'), ('MySQL User 2');
EOF

# Seed MariaDB
echo "Seeding MariaDB..."
docker exec -i $(docker compose -f tests/docker-compose.yaml ps -q mariadb) mariadb -uuser -ppassword testdb <<EOF
CREATE TABLE IF NOT EXISTS users (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255));
INSERT INTO users (name) VALUES ('MariaDB User 1'), ('MariaDB User 2');
EOF

# Seed SQLite
echo "Seeding SQLite..."
sqlite3 test.sqlite <<EOF
CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('SQLite User 1'), ('SQLite User 2');
EOF

# Create MinIO Bucket (using mc if available or just a simple curl/mc-like container)
echo "Creating MinIO bucket 'backups'..."
docker run --net=host --entrypoint=/bin/sh minio/mc -c "
  while ! nc -z localhost 9000; do sleep 1; done;
  mc alias set myminio http://localhost:9000 minioadmin minioadmin;
  mc mb myminio/backups;
" 2>/dev/null

echo "Done seeding databases and preparing MinIO!"

.SILENT:

COMPILED_FILE="dbackup"
BIN="bin"
PG_CONTAINER="dbackup-postgres"
PG_IMAGE="postgres:latest"

build:
	go build -o ./$(BIN)/$(COMPILED_FILE) main.go

run: build
	./$(BIN)/$(COMPILED_FILE)

test:
	go test ./...

check-deps:
	@which pg_dump > /dev/null || (echo "Error: pg_dump not found. Use 'make install-pg-client' to fix." && exit 1)
	@which docker > /dev/null || (echo "Error: docker not found." && exit 1)
	echo "All dependencies found."

install-pg-client:
	# 1. Clean up potential broken states
	-sudo rm -rf /var/lib/apt/lists/*
	-sudo apt-get clean
	-sudo dpkg --configure -a
	# 2. Add official PostgreSQL repository for current Debian/Ubuntu versions
	sudo apt-get update
	sudo apt-get install -y curl ca-certificates gnupg
	curl -sS https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/postgresql.gpg --yes
	# Detect codename and add repo
	CODENAME=$$(lsb_release -cs 2>/dev/null || echo "trixie"); \
	echo "deb http://apt.postgresql.org/pub/repos/apt $$CODENAME-pgdg main" | sudo tee /etc/apt/sources.list.d/pgdg.list
	# 3. Final update and install
	sudo apt-get update
	sudo apt-get install -y postgresql-client

pg-up:
	docker run --name $(PG_CONTAINER) \
		-e POSTGRES_USER=testuser \
		-e POSTGRES_PASSWORD=testpassword \
		-e POSTGRES_DB=testdb \
		-p 5432:5432 -d $(PG_IMAGE)
	echo "Waiting for Postgres to start..."
	sleep 5

pg-down:
	docker stop $(PG_CONTAINER) || true
	docker rm $(PG_CONTAINER) || true

clean:
	rm -rf ./$(BIN)
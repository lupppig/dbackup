.SILENT:


COMPILED_FILE="dbackup"
BIN="bin"

build:
	go build -o ./$(BIN)/$(COMPILED_FILE) main.go

run: build
	./$(BIN)/$(COMPILED_FILE)

test:
	go test ./...


clean:
	rm -rf ./$(BIN)
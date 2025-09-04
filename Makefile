APP_NAME=music-box-backend
BIN=tmp/main

# Default task
.DEFAULT_GOAL := run

# Run with hot reload
run:
	@air

# Build binary
build:
	go build -o $(BIN) ./cmd/app

# Run without hot reload
start: build
	./$(BIN)

# Clean build files
clean:
	rm -rf tmp

# Run tests
test:
	go test ./... -v

# Format & lint
fmt:
	go fmt ./...
	go vet ./...

# Generate sqlc code
sqlc:
	sqlc generate

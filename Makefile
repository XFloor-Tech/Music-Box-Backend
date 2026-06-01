APP_NAME=music-box-backend
BIN=bin/api

# Default task
.DEFAULT_GOAL := run

# Run with hot reload
run:
	go tool air

# Build binary
build:
	go build -o $(BIN) ./cmd/api

# Run without hot reload
start: build
	./$(BIN)

# Clean build files
clean:
	rm -rf bin

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

# Generate swagger docs
swagger:
	swag init -g cmd/api/main.go

.PHONY: build test lint fmt check-fmt clean

# Build orchestrator binary
build:
	@mkdir -p bin
	go build -o bin/orchestrator ./go/cmd/orchestrator/main.go

# Run all unit tests
test:
	go test ./go/...

# Run golangci-lint
lint:
	golangci-lint run

# Format Go source files
fmt:
	gofmt -s -w go/

# Check formatting (fails if files need formatting)
check-fmt:
	@test -z "$$(gofmt -s -l go/)" || { echo "The following files need formatting:"; gofmt -s -l go/; exit 1; }

# Remove build artifacts
clean:
	rm -rf bin/

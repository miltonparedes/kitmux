default:
    @just --list

# Build the binary
build:
    go build -o kitmux .

# Run with a flag (e.g. just run -- -p)
run *args:
    go run . {{args}}

# Run tests
test:
    go test ./...

# Run linter
lint:
    golangci-lint run

# Format code
fmt:
    golangci-lint run --fix

# Run all checks (lint, format, tests)
check:
    golangci-lint run
    go test ./...

# Tidy modules
tidy:
    go mod tidy

# Build + install to $GOPATH/bin
install:
    go install .

# Clean build artifacts
clean:
    rm -f kitmux

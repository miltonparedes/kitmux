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

# Install git hooks (pre-commit / pre-push) via lefthook
hooks-install:
    lefthook install

# Uninstall git hooks
hooks-uninstall:
    lefthook uninstall

# Report top-20 most complex functions (non-blocking)
complexity-report:
    @command -v gocyclo >/dev/null 2>&1 || go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
    @gocyclo -top 20 -avg .

# --- Database dev tools ---

db_path := env("HOME") / ".config/kitmux/state.db"

# Delete the state DB (re-created on next run)
db-reset:
    rm -f {{db_path}} && echo "Removed {{db_path}}"

# Show current schema version
db-version:
    sqlite3 {{db_path}} "PRAGMA user_version;"

# Open an interactive SQLite shell
db-shell:
    sqlite3 {{db_path}}

# Dump the database schema (DDL)
db-schema:
    sqlite3 {{db_path}} ".schema"

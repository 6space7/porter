GO ?= go
SQLC ?= github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: test build generate migrate verify

test:
	$(GO) test ./... -count=1

build:
	$(GO) build ./cmd/server

generate:
	$(GO) run $(SQLC) generate

migrate:
	$(GO) test ./internal/store -run 'TestOpenMigratesInitialSchema|TestOpenSeedsLocalServerIdempotently' -count=1

verify: test build
	bash -n install.sh

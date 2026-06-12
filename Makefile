GO ?= go
SQLC ?= github.com/sqlc-dev/sqlc/cmd/sqlc@latest
NPM ?= npm

.PHONY: test build frontend-check frontend-build generate migrate verify

test:
	$(GO) test ./... -count=1

build:
	$(GO) build ./cmd/server

frontend-check:
	cd frontend && $(NPM) run check

frontend-build:
	cd frontend && $(NPM) run build

generate:
	$(GO) run $(SQLC) generate

migrate:
	$(GO) test ./internal/store -run 'TestOpenMigratesInitialSchema|TestOpenSeedsLocalServerIdempotently' -count=1

verify: frontend-check frontend-build test build
	bash -n install.sh

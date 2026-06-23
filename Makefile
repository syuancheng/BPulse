SHELL := /bin/sh

-include .env.local
export

APP_ENV ?= local
PORT ?= 8080
MYSQL_DSN ?= bp_companion:bp_companion_local@tcp(127.0.0.1:3306)/bp_companion?parseTime=true&charset=utf8mb4&loc=UTC
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod

.PHONY: fmt fmt-check lint unit integration test build secret-scan verify run-api migrate-up migrate-down-one

fmt:
	find server -name '*.go' -type f -exec gofmt -w {} +

fmt-check:
	./scripts/check-format.sh

lint:
	./scripts/lint.sh

unit:
	go test ./...
	npm --prefix media-parser test
	npm --prefix reminder-worker test

integration:
	./scripts/test-integration.sh

test: unit integration

build:
	./scripts/build.sh

secret-scan:
	./scripts/secret-scan.sh

verify: fmt-check lint unit integration build secret-scan

run-api:
	go run ./server/cmd/api

migrate-up:
	go run ./server/cmd/migrate -direction up

migrate-down-one:
	go run ./server/cmd/migrate -direction down-one

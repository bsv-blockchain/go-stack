SHELL=/bin/bash

GIT_COMMIT_SHORT := $(shell git rev-parse --short HEAD)
VERSION ?= MANUAL

.PHONY: all
all: deps lint build test

.PHONY: deps
deps:
	GOPRIVATE=github.com/teranode-group go mod tidy

.PHONY: build
build:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o artifacts/svc  -ldflags="-s -w -X main.commit=$(GIT_COMMIT_SHORT) -X main.version=$(VERSION)" .

.PHONY: unit_test
unit_test:
	go test -v -count=1 $(go list ./... | grep -v /system_test/)

.PHONY: lint
lint:
	golangci-lint run --fix

.PHONY: run
run:
	go run main.go | zap-pretty

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/alexremn/kubectl-fieldlord/internal/buildinfo
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.PHONY: build test lint cover third-party-licenses
build:
	CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o bin/kubectl-fieldlord ./cmd/kubectl-fieldlord
test:
	go test -race ./...
cover:
	go test -race -coverprofile=coverage.txt ./...
	go tool cover -func=coverage.txt | tail -1
lint:
	golangci-lint run
third-party-licenses:
	go run github.com/google/go-licenses@latest report ./... > THIRD_PARTY_LICENSES 2>/dev/null || true

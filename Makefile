VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
COMMIT  ?= $(shell git rev-parse --short=7 HEAD 2>/dev/null)
DATE    ?= $(shell date -u +%Y-%m-%d)
PKG      = github.com/RafaSG13/Pandora/internal/version
LDFLAGS  = -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/pandora ./cmd/pandora

run:
	go run ./cmd/pandora

vet:
	go vet ./...

test:
	go test ./...

.PHONY: build run vet test

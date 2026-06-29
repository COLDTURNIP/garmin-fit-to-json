GO ?= go
VERSION ?= dev
BINARY ?= dist/fit2json

.PHONY: validate fmt-check vet test build clean

validate: fmt-check vet test build

fmt-check:
	@files="$$(gofmt -l .)"; \
	if test -n "$$files"; then \
		printf 'gofmt needed:\n%s\n' "$$files"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

build:
	mkdir -p dist
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/fit2json


clean:
	rm -rf dist

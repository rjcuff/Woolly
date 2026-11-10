VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install web-test check release

build:
	go build -ldflags "$(LDFLAGS)" -o bin/woolly ./cmd/woolly

test:
	go vet ./... && go test ./...

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/woolly

# Type-checks the site, builds it, and runs the Playwright tests against the build.
web-test:
	cd web && npm run types:check && npm run test:e2e

# Run this before every commit.
check: test web-test

# make release VERSION=0.1.1
release: check
	scripts/release.sh $(VERSION)

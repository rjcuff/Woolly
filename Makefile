VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install ui web-test ui-test check release publish

build:
	go build -ldflags "$(LDFLAGS)" -o bin/woolly ./cmd/woolly

test:
	go vet ./... && go test ./...

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/woolly

# Rebuilds the review page that gets embedded in the binary.
ui:
	cd ui && npm run build

# Browser tests for the review page, against a real woolly server.
ui-test:
	cd ui && npm run test:e2e

# Type-checks the site, builds it, and runs the Playwright tests against the build.
web-test:
	cd web && npm run types:check && npm run test:e2e

# Run this before every commit.
check: test web-test ui-test

# make release VERSION=0.1.1
release: check
	scripts/release.sh $(VERSION)

# make publish VERSION=0.1.1  (tests, builds, tags, pushes and uploads the release)
publish: release
	scripts/publish.sh $(VERSION)

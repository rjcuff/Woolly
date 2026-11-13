VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install ui web-test ui-test py-test js-test check release publish demo pypi npm app

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
# Tests the Python package against a real woolly server.
py-test: build
	cd sdk/python && python3 -m unittest discover -s tests

# Tests the JavaScript package against a real woolly server.
js-test: build
	cd sdk/js && { [ -d node_modules ] || npm install --no-audit --no-fund; } && npm test

check: test web-test ui-test py-test js-test

# make release VERSION=0.1.1
release: check
	scripts/release.sh $(VERSION)

# make publish VERSION=0.1.1  (tests, builds, tags, pushes and uploads the release)
# On a Mac it also builds, signs and notarizes the Mac app and attaches it.
publish: release
	if [ "$$(uname -s)" = Darwin ]; then \
		scripts/app.sh $(VERSION) && cp dist-app/Woolly-mac.dmg dist/ && (cd dist && shasum -a 256 Woolly-mac.dmg >> checksums.txt); \
	fi
	scripts/publish.sh $(VERSION)

# Records docs/demo.gif from real woolly output.
demo: build
	cd web && node scripts/demo.ts

# Builds and uploads the woolly-ai Python package. Bump the version in
# sdk/python/pyproject.toml and sdk/python/woolly/__init__.py first.
pypi: py-test
	cd sdk/python && rm -rf dist && python3 -m build && python3 -m twine upload dist/*

# Builds the desktop app for this computer. Needs Rust (rustup.rs).
app:
	scripts/app.sh $(VERSION)

# Publishes the woolly-ai npm package. Bump the version in sdk/js/package.json first.
npm: js-test
	cd sdk/js && npm publish --access public

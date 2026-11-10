VERSION ?= 0.1.0-dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test install

build:
	go build -ldflags "$(LDFLAGS)" -o bin/woolly ./cmd/woolly

test:
	go vet ./... && go test ./...

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/woolly

# AGENTS.md

## Building

```sh
make build        # bin/woolly
make check        # Go tests, site tests, review page tests and Python tests
```

Quick Go-only iteration:

```sh
go run ./cmd/woolly doctor
WOOLLY_HOME=$(mktemp -d) go run ./cmd/woolly serve
```

Set `WOOLLY_HOME` to a temporary folder when testing so you never touch real memories.

## Layout

- `cmd/woolly`: the command line
- `internal/`: memory store, search, MCP server, local API, proxy, app connections
- `ui/`: the review page, built into the binary with `make ui`
- `web/`: the website and docs at usewoolly.com, from `docs/*.mdx`
- `sdk/python`: the `woolly-ai` package

## Rules

- No new Go dependencies.
- Web code is TypeScript and TSX.
- When you add or change a feature, update the docs, README and CHANGELOG.
- Write in plain American English. No em dashes.

See `CONTRIBUTING.md` for more.

# Woolly

**Ollama runs your models. Woolly remembers for them.**

Woolly is a local memory for your AI tools. Claude, Cursor, ChatGPT and your own scripts
can all read and write the same memory, and it lives on your laptop as plain Markdown files you own.

- **One binary, no setup.** No Docker, no database server, no account.
- **Files you can read.** Every memory is a `.md` file in `~/.woolly/memories`. Edit it, grep it, back it up, put it in git.
- **Memories don't go stale.** A newer fact with the same key replaces the old one, so "moved to Denver" wins over "lives in Austin".
- **You stay in charge.** If an AI guesses something that contradicts what *you* said, it's saved as a conflict for you to review instead of overwriting you.
- **Works with any MCP app.** Claude Code, Claude Desktop, Cursor and others.
- **Better with Ollama, but doesn't need it.** If Ollama is running, Woolly adds meaning-based search; otherwise it uses keyword search.

## Install

```sh
curl -fsSL https://woolly.sh/install.sh | sh
# or
go install github.com/woolly/woolly/cmd/woolly@latest
```

Full docs live in [`docs/`](docs) and at [woolly.sh/docs](https://woolly.sh/docs).

## Use it

```sh
woolly remember "I prefer tabs over spaces" --key code.indent
woolly remember "Moved to Denver in October" --key user.location
woolly recall "where do I live"
woolly list
woolly conflicts        # review disagreements
woolly resolve <id>     # keep the right one
woolly doctor           # check your setup
```

## Connect your AI tools

**Claude Code**

```sh
claude mcp add woolly -- woolly mcp
```

**Claude Desktop / Cursor**: add this to the app's MCP config:

```json
{ "mcpServers": { "woolly": { "command": "woolly", "args": ["mcp"] } } }
```

## Semantic search (optional)

Install [Ollama](https://ollama.com), then:

```sh
ollama pull nomic-embed-text
woolly doctor   # should say "semantic + keyword"
```

## HTTP API

`woolly serve` listens on `127.0.0.1:11435` (next to Ollama's 11434) and only accepts local requests.

| Method | Path | Body |
| --- | --- | --- |
| POST | `/api/remember` | `{"content", "key"?, "tags"?, "source"?}` |
| POST | `/api/recall` | `{"query", "limit"?, "include_all"?}` |
| GET | `/api/memories?all=1` | |
| GET / DELETE | `/api/memories/{id}` | |
| GET | `/api/conflicts` | |
| POST | `/api/resolve` | `{"keep_id"}` |

## How freshness works

| Situation | What happens |
| --- | --- |
| Same text again | Treated as a duplicate; the timestamp is refreshed |
| New fact, same `key` | The old memory is marked `superseded` and hidden from recall |
| An AI contradicts something you said | Saved as `conflict`; your fact stays current until you `resolve` it |

Trust order: `user` > `import` > `agent`.

## Configuration

| Variable | Default |
| --- | --- |
| `WOOLLY_HOME` | `~/.woolly` |
| `WOOLLY_HOST` | `127.0.0.1:11435` |
| `OLLAMA_HOST` | `127.0.0.1:11434` |
| `WOOLLY_EMBED_MODEL` | `nomic-embed-text` |
| `WOOLLY_NO_EMBED` | unset (set to disable Ollama) |

## Repo layout

| Folder | What's in it |
| --- | --- |
| `cmd/woolly`, `internal/` | The Woolly program (Go, no dependencies) |
| `docs/` | Documentation as MDX, rendered by the website |
| `web/` | Website and docs site (Fumadocs on React Router and Vite) |
| `scripts/install.sh` | The `curl \| sh` installer |

Run the website locally with `cd web && npm install && npm run dev`.

## License

MIT

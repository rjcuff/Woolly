# Woolly

Ollama runs your models. Woolly remembers for them.

Woolly is a memory for your AI tools that runs on your own computer. Claude, Cursor and your own scripts all read and write the same memory, and it's stored as plain Markdown files you can open and edit.

- One binary. No Docker, no database, no account.
- Every memory is a `.md` file in `~/.woolly/memories`.
- When a fact changes, the new one replaces the old one.
- If an AI contradicts something you said, your version stays and the AI's is flagged for review.
- Works with Claude Code, Claude Desktop, Cursor and any other MCP app.
- Ollama is optional. With it, search also matches by meaning.

## Install

```sh
curl -fsSL https://woolly.sh/install.sh | sh
```

Or with Go:

```sh
go install github.com/woolly/woolly/cmd/woolly@latest
```

## Use it

```sh
woolly remember "I prefer tabs over spaces" --key code.indent
woolly remember "Moved to Denver in October" --key user.location
woolly recall "where do I live"
```

Bring in what you've already told ChatGPT or Claude:

```sh
woolly import chatgpt ~/Downloads/chatgpt-export.zip --dry-run
```

## Connect Claude Code

```sh
claude mcp add woolly -- woolly mcp
```

For Claude Desktop and Cursor, add this to the app's MCP config:

```json
{ "mcpServers": { "woolly": { "command": "woolly", "args": ["mcp"] } } }
```

## Docs

Read them at [woolly.sh/docs](https://woolly.sh/docs) or in the [`docs`](docs) folder.

## Repo layout

| Folder | Contents |
| --- | --- |
| `cmd/woolly`, `internal` | The Woolly program, written in Go |
| `docs` | Documentation in MDX |
| `web` | Website and docs site |
| `scripts` | The install script |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT

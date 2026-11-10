# Woolly

Ollama runs your models. Woolly remembers for them.

![Woolly saving, replacing and recalling memories in a terminal](docs/demo.gif)

Woolly is a memory for your AI tools that runs on your own computer. Claude, Cursor and your own scripts all read and write the same memory, and it's stored as plain Markdown files you can open and edit.

- One binary. No Docker, no database, no account.
- Every memory is a `.md` file in `~/.woolly/memories`.
- When a fact changes, the new one replaces the old one.
- If an AI contradicts something you said, your version stays and the AI's is flagged for review.
- Passwords, API keys and card numbers are never saved, and an AI can't plant instructions in your memory.
- Forgotten memories go to a trash you can restore from.
- Works with Claude Code, Claude Desktop, Cursor and any other MCP app.
- `woolly proxy` gives memory to Ollama and to any app that uses the OpenAI or Anthropic API.
- Ollama is optional. With it, search also matches by meaning.

## Install

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://usewoolly.com/install.ps1 | iex
```

Or with Go:

```sh
go install github.com/rjcuff/woolly/cmd/woolly@latest
```

## Use it

```sh
woolly remember "I prefer tabs over spaces" --key code.indent
woolly remember "Moved to Denver in October" --key user.location
woolly recall "where do I live"
woolly profile      # summary of what Woolly knows about you
woolly ui           # review and edit memories in your browser
woolly forget <id>  # moves it to the trash; undo with woolly restore <id>
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

## Give Ollama a memory

```sh
woolly proxy
OLLAMA_HOST=127.0.0.1:11436 ollama run llama3.2
```

Every chat now includes what Woolly knows about you, and facts you mention are saved. It also works with OpenAI and Anthropic apps. See [Proxy](https://usewoolly.com/docs/proxy).

## Docs

Read them at [usewoolly.com/docs](https://usewoolly.com/docs) or in the [`docs`](docs) folder.

## Repo layout

| Folder | Contents |
| --- | --- |
| `cmd/woolly`, `internal` | The Woolly program, written in Go |
| `docs` | Documentation in MDX |
| `ui` | The review page, built into the binary |
| `web` | Website and docs site |
| `scripts` | Install, release and Homebrew scripts |
| `packaging/homebrew` | The Homebrew formula, kept in sync with the tap |

## Security

See [SECURITY.md](SECURITY.md) and [usewoolly.com/docs/security](https://usewoolly.com/docs/security).

## Changes

See [CHANGELOG.md](CHANGELOG.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT

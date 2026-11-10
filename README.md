<div align="center">

# Woolly

**One memory for all your AI tools. Local, private, and always up to date.**

Ollama runs your models. Woolly remembers for them.

[![Release](https://img.shields.io/github/v/release/rjcuff/woolly?color=111111)](https://github.com/rjcuff/woolly/releases/latest)
[![PyPI](https://img.shields.io/pypi/v/woolly-ai?color=111111&label=pypi)](https://pypi.org/project/woolly-ai/)
[![License: MIT](https://img.shields.io/badge/license-MIT-111111)](LICENSE)

[Website](https://usewoolly.com) · [Docs](https://usewoolly.com/docs) · [Quickstart](https://usewoolly.com/docs/quickstart)

![Woolly saving, replacing and recalling memories in a terminal](docs/demo.gif)

</div>

Every AI app forgets you, or keeps its own memory on someone else's servers. Claude doesn't know what you told ChatGPT, and Cursor doesn't know either.

Woolly runs on your computer and gives all of them one shared memory. It's stored as plain Markdown files you can open and edit, and nothing ever leaves your machine.

## Get started

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
woolly remember "I prefer short answers"
claude mcp add woolly -- woolly mcp
```

Now Claude Code remembers. So does every other app you connect.

Other ways to install: `brew install rjcuff/tap/woolly`, or on Windows `irm https://usewoolly.com/install.ps1 | iex`, or `go install github.com/rjcuff/woolly/cmd/woolly@latest`.

## What it does

**Works with your AI tools.** Claude Code, Claude Desktop, Cursor and any other MCP app can save and look up memories on their own.

**Gives Ollama a memory.** `woolly proxy` adds what Woolly knows to every chat, for Ollama and any app that uses the OpenAI or Anthropic API.

**Keeps facts current.** Say you moved to Denver, and Denver replaces Austin. Your AI stops bringing up things that stopped being true.

**Lets you decide.** If an AI guesses something that contradicts what you said, your version stays and the guess is flagged for you to review.

**Shows you everything.** `woolly ui` opens a page where you can search, edit and delete every memory. Or open the files yourself.

**Brings in your history.** `woolly import` pulls facts about you out of your ChatGPT and Claude exports.

**Stays private and safe.** No cloud, no account, no tracking. Passwords and API keys are refused, an AI can't plant instructions in your memory, and deleted memories go to a trash you can restore from.

**Builds into your apps.** `pip install woolly-ai` and add memory to any prompt in one line.

## Give Ollama a memory

```sh
woolly proxy
OLLAMA_HOST=127.0.0.1:11436 ollama run llama3.2
```

Every chat now includes what Woolly knows about you, and facts you mention are saved. For OpenAI or Anthropic apps, point the app's base URL at the proxy. See [Proxy](https://usewoolly.com/docs/proxy).

## Use it from Python

```sh
pip install woolly-ai
```

```python
import woolly

woolly.remember("Moved to Denver in October", key="user.location")
system_prompt = "You are a helpful assistant.\n\n" + woolly.context(user_message)
```

See [Python](https://usewoolly.com/docs/python).

## Commands

```sh
woolly remember "Moved to Denver" --key user.location   # a newer fact with the same key replaces the old one
woolly recall "where do I live"
woolly profile          # a summary of what Woolly knows about you
woolly ui               # review and edit memories in your browser
woolly import chatgpt ~/Downloads/export.zip --dry-run
woolly forget <id>      # moves it to the trash; undo with woolly restore <id>
woolly proxy            # memory for Ollama, OpenAI and Anthropic apps
```

Full list: [Commands](https://usewoolly.com/docs/cli).

## Connect Claude Desktop and Cursor

Add this to the app's MCP config:

```json
{ "mcpServers": { "woolly": { "command": "woolly", "args": ["mcp"] } } }
```

See [Connect your apps](https://usewoolly.com/docs/connect).

## Do I need Ollama?

No. Woolly works on its own. If Ollama is running, Woolly also uses it to search by meaning.

## Repo layout

| Folder | Contents |
| --- | --- |
| `cmd/woolly`, `internal` | The Woolly program, written in Go |
| `docs` | Documentation in MDX |
| `ui` | The review page, built into the binary |
| `sdk/python` | The `woolly-ai` Python package |
| `web` | Website and docs site |
| `scripts` | Install, release and Homebrew scripts |
| `packaging/homebrew` | The Homebrew formula, kept in sync with the tap |

## More

- [Security](SECURITY.md) and [how Woolly protects your data](https://usewoolly.com/docs/security)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)

MIT license.

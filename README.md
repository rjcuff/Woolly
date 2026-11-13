<div align="center">

# Woolly

**One memory for all your AI tools. Local, private, and always up to date.**

Ollama runs your models. Woolly remembers for them.

[![Release](https://img.shields.io/github/v/release/rjcuff/woolly?color=111111)](https://github.com/rjcuff/woolly/releases/latest)
[![PyPI](https://img.shields.io/pypi/v/woolly-ai?color=111111&label=pypi)](https://pypi.org/project/woolly-ai/)
[![License: MIT](https://img.shields.io/badge/license-MIT-111111)](LICENSE)

[Website](https://usewoolly.com) · [Docs](https://usewoolly.com/docs) · [Quickstart](https://usewoolly.com/docs/quickstart) · [Bluesky](https://bsky.app/profile/usewoolly.com) · [LinkedIn](https://www.linkedin.com/company/usewoolly)

![Woolly saving notes for two projects, Claude Code loading only the acme notes at startup, and woolly log showing what each app did](docs/demo.gif)

</div>

Every AI app forgets you, or keeps its own memory on someone else's servers. Claude doesn't know what you told ChatGPT, and Cursor doesn't know either.

Woolly runs on your computer and gives all of them one shared memory. It's stored as plain Markdown files you can open and edit, and nothing ever leaves your machine.

## Get started

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
woolly remember "I prefer short answers"
woolly connect
```

`woolly connect` adds Woolly to Claude Code, Claude Desktop, Cursor, VS Code, Windsurf, Codex, Gemini CLI, LM Studio and opencode. Now they all remember.

Prefer an app? [Download Woolly for Mac](https://github.com/rjcuff/woolly/releases/latest/download/Woolly-mac.dmg). It works on Apple Silicon and Intel, and lives in your menu bar.

Using Claude Desktop? Download [woolly.mcpb](https://github.com/rjcuff/woolly/releases/latest/download/woolly.mcpb) and double-click it.

Other ways to install: `brew install rjcuff/tap/woolly`, or on Windows `irm https://usewoolly.com/install.ps1 | iex`, or `go install github.com/rjcuff/woolly/cmd/woolly@latest`.

## What it does

**Works with your AI tools.** Claude Code, Claude Desktop, Cursor and any other MCP app can save and look up memories on their own.

**Gives Ollama a memory.** `woolly proxy` adds what Woolly knows to every chat, for Ollama and any app that uses the OpenAI or Anthropic API.

**Keeps facts current.** Say you moved to Denver, and Denver replaces Austin. Your AI stops bringing up things that stopped being true.

**Lets you decide.** If an AI guesses something that contradicts what you said, your version stays and the guess is flagged for you to review.

**Loads by itself.** In Claude Code, every session starts with your profile and notes about the project you're in. No need to ask.

**Learns from your chats.** `woolly learn` uses a model running in Ollama to pick out facts from a conversation. Newer facts replace older ones, and nothing leaves your computer.

**Syncs your devices.** `woolly sync` keeps your laptop, desktop and Raspberry Pi in step over your own network. Encrypted, no cloud, no account.

**Picks up where you left off.** When Claude Code compacts or a session ends, Woolly saves what you were doing, what was decided and what's next. The next session starts there, in any app.

**Shares project memory with your team.** `woolly share` keeps a repo's project notes in `.woolly/` so everyone's agents start with the same context. Commit it like any other file. No cloud.

**Keeps projects apart.** Notes about a codebase stay with that repo, and facts about you go everywhere. Your AI in one project never gets another project's notes.

**Shows you everything.** `woolly ui` opens the Woolly app: what your AI tools know, one-click app connections, and every memory to search, edit or forget. Or open the files yourself. Every memory notes which app saved it, and `woolly log` shows what each app saved, read and deleted.

**Brings in your history.** `woolly import` pulls facts about you out of your ChatGPT and Claude exports.

**Stays private and safe.** No cloud, no account, no tracking. Passwords and API keys are refused, an AI can't plant instructions in your memory, and deleted memories go to a trash you can restore from.

**Builds into your apps.** `pip install woolly-ai` and add memory to any prompt in one line.

## Give Ollama a memory

```sh
woolly service install   # or run `woolly proxy` in a terminal
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
woolly ui               # open the Woolly app
woolly log              # what each app saved, read and deleted
woolly handoff          # where you left off in this project
woolly share            # share this repo's project memories with your team
woolly learn "I just moved to Denver"   # a local model picks out the facts
woolly sync init        # keep your devices in step
woolly import chatgpt ~/Downloads/export.zip --dry-run
woolly forget <id>      # moves it to the trash; undo with woolly restore <id>
woolly proxy            # memory for Ollama, OpenAI and Anthropic apps
woolly connect          # add Woolly to every AI app on this computer
woolly service install  # run in the background and start at login, like Ollama
```

Full list: [Commands](https://usewoolly.com/docs/cli).

## Connect your apps

```sh
woolly connect
```

Or add this to any MCP app's config yourself:

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
| `ui` | The Woolly app's screens, built into the binary |
| `app` | The Mac app (Tauri), which wraps the same screens |
| `sdk/python` | The `woolly-ai` Python package |
| `web` | Website and docs site |
| `scripts` | Install, release and Homebrew scripts |
| `packaging/homebrew` | The Homebrew formula, kept in step with the tap |
| `packaging/mcpb` | The Claude Desktop bundle |

## More

- [Security](SECURITY.md) and [how Woolly protects your data](https://usewoolly.com/docs/security)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)

MIT license.

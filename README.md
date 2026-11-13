<div align="center">

# Woolly

**One memory for all your AI tools. Local, private, and always up to date.**

Ollama runs your models. Woolly remembers for them.

[![Release](https://img.shields.io/github/v/release/rjcuff/woolly?color=111111)](https://github.com/rjcuff/woolly/releases/latest)
[![PyPI](https://img.shields.io/pypi/v/woolly-ai?color=111111&label=pypi)](https://pypi.org/project/woolly-ai/)
[![npm](https://img.shields.io/npm/v/woolly-ai?color=111111&label=npm)](https://www.npmjs.com/package/woolly-ai)
[![License: MIT](https://img.shields.io/badge/license-MIT-111111)](LICENSE)

<a href="https://www.producthunt.com/products/woolly?embed=true&utm_source=badge-featured&utm_medium=badge&utm_campaign=badge-woolly" target="_blank" rel="noopener noreferrer"><picture><source media="(prefers-color-scheme: dark)" srcset="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1315543&theme=dark"><img alt="Woolly - One local memory for all your AI tools | Product Hunt" width="250" height="54" src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1315543&theme=light"></picture></a>

[Website](https://usewoolly.com) · [Docs](https://usewoolly.com/docs) · [Download for Mac and Windows](https://usewoolly.com/download) · [Bluesky](https://bsky.app/profile/usewoolly.com) · [LinkedIn](https://www.linkedin.com/company/usewoolly)

![Woolly saving notes for two projects, Claude Code loading only the acme notes at startup, and woolly log showing what each app did](docs/demo.gif)

</div>

Every AI app forgets you, or keeps its own memory on someone else's servers. Claude doesn't know what you told ChatGPT. Cursor doesn't know what you decided in Claude Code. And your teammates' agents start from zero every time.

Woolly gives all of them one memory. It runs on your computer, it's stored as Markdown files you can read, and nothing leaves your machine.

## Get started

```sh
curl -fsSL https://usewoolly.com/install.sh | sh
woolly connect
```

That's it. Claude Code, Claude Desktop, Cursor, VS Code, Windsurf, Codex, Gemini CLI, LM Studio and opencode now share one memory.

Prefer an app? [Download Woolly for Mac or Windows](https://usewoolly.com/download). Also on [Homebrew and Linux](https://usewoolly.com/docs/quickstart).

## Your team's AI shares one project memory

```sh
woolly init
```

Run it in a repo and commit `.woolly/`. Now every teammate's agent reads and updates the same project memory: the stack, conventions, decisions, gotchas and commands. It's a CLAUDE.md that keeps itself up to date.

- **Decisions are saved for you.** When a Claude Code session ends, Woolly saves what you decided, why, and what you turned down, so no agent suggests it again.
- **Facts stay current.** A newer fact replaces the old one instead of piling up.
- **Personal stays personal.** Facts about you never go in the repo.
- **Plain Markdown in git.** One fact per line, readable diffs, edit it by hand.

[Project memory →](https://usewoolly.com/docs/project-memory)

## Why Woolly

**Every app, one memory.** Tell Claude you moved to Denver, and Cursor knows it too. [How it works →](https://usewoolly.com/docs/how-it-works)

**Memory that loads by itself.** Claude Code starts every session with your profile, the project's notes and where you left off. [Connect →](https://usewoolly.com/docs/connect)

**Always up to date.** When a fact changes, the new one replaces the old. When an AI guesses something that contradicts you, your version wins and the guess is flagged for review.

**Gives Ollama a memory.** `woolly proxy` adds memory to every chat with Ollama, or any app that uses the OpenAI or Anthropic API. [Proxy →](https://usewoolly.com/docs/proxy)

**Brings your memory with you.** Drop in your ChatGPT or Claude export and keep the facts that matter. [Import →](https://usewoolly.com/docs/import)

**Private by design.** No cloud, no account, no tracking. Passwords and API keys are refused, and an AI can't plant instructions in your memory. [Security →](https://usewoolly.com/docs/security)

**See everything.** The Woolly app shows what your AI tools know and which app saved each memory. Search, edit or forget anything. [The app →](https://usewoolly.com/docs/review)

**Syncs your devices.** Laptop, desktop and Raspberry Pi stay in step over your own network, encrypted. [Sync →](https://usewoolly.com/docs/sync)

## Build it into your app

```sh
npm install woolly-ai    # or: pip install woolly-ai
```

```ts
import { Woolly } from 'woolly-ai';

const w = new Woolly({ app: 'My app' });
const system = `You are a helpful assistant.\n\n${await w.context(userMessage)}`;
```

One line adds memory to the Vercel AI SDK too. [JavaScript →](https://usewoolly.com/docs/javascript) · [Python →](https://usewoolly.com/docs/python)

## Fast and accurate, on a laptop

On a sample of the [LOCOMO](https://github.com/snap-research/locomo) long-conversation benchmark, Woolly found the right memory for 87% of questions in a median of 23 ms, running entirely on a laptop with Ollama. [Run it yourself →](https://usewoolly.com/docs/benchmark)

## Do I need Ollama?

No. Woolly works on its own. With Ollama running, it also searches by meaning, learns facts from your chats and saves decisions when a session ends.

## Learn more

- [Docs](https://usewoolly.com/docs) and [all commands](https://usewoolly.com/docs/cli)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md), including how the repo is laid out
- [Security](SECURITY.md)

MIT license.

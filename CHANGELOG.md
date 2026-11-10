# Changelog

## Unreleased

- `woolly proxy` gives memory to Ollama and to apps that use the OpenAI or Anthropic API. It adds relevant memories to each chat and saves facts you state.
- Windows install script: `irm https://usewoolly.com/install.ps1 | iex`.
- Homebrew formula, published to `rjcuff/homebrew-tap` on every release.

## 0.3.0

### Security

- Text from an AI or an import that reads like instructions to an AI is refused, so a malicious web page can't plant commands in your memory.
- Recalled memories are labeled as notes, not instructions, when sent to AI apps.
- The local API now requires a secret token stored in `~/.woolly/token`. `woolly ui` handles it for you.
- The local API rejects requests that browsers mark as coming from another site.
- The install script verifies downloads against the release checksums.
- Imports refuse files larger than 2 GB.

### New

- Forgetting a memory moves it to the trash. Use `woolly trash` to see it, `woolly restore <id>` to bring it back, and `woolly trash --empty` to delete for good.
- The review page has a Trash tab and an Undo button after you forget something.
- New Security page in the docs, and SECURITY.md.

### Changed

- Docs use a simpler layout with a top bar, a plain FAQ, and no numbered steps.
- `go install` builds now report the right version.
- The install script creates its install folder if it doesn't exist.

## 0.2.1

- The project moved to github.com/rjcuff/woolly and usewoolly.com. Old links redirect.

## 0.2.0

- `woolly ui` opens a review page to add, search, edit and forget memories, resolve conflicts and copy your profile.
- `woolly profile` and the `get_profile` tool give a short summary of what Woolly knows about you.
- Passwords, API keys and other secrets are refused.

## 0.1.0

- First release: remember, recall, list, forget, conflicts and resolve.
- `woolly import` for ChatGPT and Claude exports.
- `woolly mcp` for Claude Code, Claude Desktop, Cursor and other MCP apps.
- `woolly serve` for a local HTTP API.
- Optional search by meaning with Ollama.

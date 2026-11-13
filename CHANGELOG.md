# Changelog

## 0.12.0

- Import is smarter with Ollama. When the learning model is installed, Woolly has it read your recent messages about yourself and combines what it finds with the simple rules. In testing it found every fact in a sample export, up from about half.

- Import from ChatGPT and Claude in the app. Drop your data export into Memories → Import, review the facts Woolly found, uncheck what you don't want, and save. It works out which app the export came from, marks what Woolly already knows, and labels imported memories with where they came from. Also in the API: `POST /api/import/preview` and `/api/import/save`.
- Exports saved without a `.zip` name are now read as zips.

- Decisions. `woolly decide` and the new `decide` MCP tool save a choice with the reason and the options that were turned down. Sessions start with an Already decided list, and agents are told not to suggest a rejected option again. The Memories tab can filter to decisions.
- Notes that know when they're out of date. Project notes saved in a git repo record the commit, and the startup context marks a note when files it mentions changed since.

## 0.11.0

- The Mac app is signed and notarized by Apple, so it opens with a double-click. [Download it](https://github.com/rjcuff/woolly/releases/latest/download/Woolly-mac.dmg). One download works on Apple Silicon and Intel.

- Pick up where you left off. When Claude Code compacts a conversation or a session ends, Woolly saves a handoff note for the project: what you were working on, what got done, what was decided and what's next. The next session starts with it, in Claude Code or any connected app. A quick note is saved instantly, then a local model writes a better one in the background when Ollama is running. `woolly handoff` shows it, and MCP apps can save one with the new `handoff` tool. Run `woolly connect claude-code` again to add the new hooks.
- Share project memory with your team. `woolly share` keeps a repo's project notes in `.woolly/memories` as plain files to commit. Teammates' AI tools start with the same notes, and new notes are written back as they're saved. Only project notes are shared, never personal ones, and notes from a repo are checked for secrets and injected instructions. `--agents-md` also keeps a Project notes section in AGENTS.md for tools without MCP.

## 0.10.0

- The Woolly app. `woolly ui` opens a new app with Home (your memories at a glance, with a chart of what apps saved each day), Memories (add, search, edit in place, forget), Apps (connect Claude, Cursor, VS Code and more with one click) and Settings (Ollama, sync, conflicts, history, trash and activity).
- A Mac app in preview wraps the same screens, with a menu bar icon. Download `Woolly-mac.dmg` from the release. It isn't signed yet, so the first time you open it, go to System Settings → Privacy & Security and choose Open Anyway.
- `woolly learn` no longer saves anything from a message that only asks questions.
- New API endpoints for the app: `GET /api/apps`, `POST /api/apps/{id}/connect` and `/disconnect`, `GET /api/settings`, and `POST /api/sync/init`, `/join`, `/now`, `/leave`.

## 0.9.0

- Smart memory. `woolly learn` uses a model running in Ollama to pick out facts from a conversation or any text. A fact on the same subject as an older one replaces it, like a new city or a new editor, and repeats are skipped. Facts are saved as things an AI figured out, so they never overrule what you said yourself, and secrets and instructions are still refused. Also in the API (`POST /api/learn`) and Python (`w.learn(...)`). The default model is `qwen2.5:3b`; set `WOOLLY_LLM_MODEL` to change it. See [Smart memory](https://usewoolly.com/docs/learn).
- The proxy uses the model to save facts from your chats when it's available.
- Sync between your devices. `woolly sync init` and `woolly sync join` link your laptop, desktop or Raspberry Pi over your own network, with no cloud. Every message is encrypted and signed with a key only your devices have. The newest change wins, and memories deleted for good stay deleted everywhere. `woolly serve` and the background service sync every 5 minutes. See [Sync your devices](https://usewoolly.com/docs/sync).
- Builds for 32-bit ARM, for Raspberry Pi OS 32-bit. The install script picks it automatically.
- `woolly doctor` shows whether learning and sync are ready.
- Forgetting and restoring a memory now update its time, so sync knows it's the newest change.
- Python package 0.3.0 adds `learn()`.

## 0.8.0

- Claude Desktop can install Woolly with a double-click: download `woolly.mcpb` from the latest release. It holds Woolly for macOS, Windows and Linux.
- Woolly is listed on the official MCP Registry as `io.github.rjcuff/woolly`.
- Projects. Facts about a codebase can belong to that project, and facts about you stay global. Apps working in a git repo see your memories plus that project's, and nothing from other projects. Keys only clash within a project. Use `--project` on `remember`, `recall` and `list`, `project` in the API and Python package, and the project picker on the review page. See [Projects](https://usewoolly.com/docs/projects).
- The profile only lists facts about you. Project notes are shown separately.

## 0.7.0

- Memory that loads by itself. `woolly connect` adds a startup hook to Claude Code, so each session starts with your profile and the notes about the project you're in. `woolly context` prints what it loads. `woolly disconnect` removes the hook.
- Every memory records the app that saved it, like Claude Code, Cursor, the proxy or the command line. The review page shows it as "via Cursor".
- `woolly log` shows what each app saved, read and deleted. It's also on the new **Activity** tab of the review page and at `GET /api/activity`. The log stays in `~/.woolly/activity.log`.
- Apps using the local API can name themselves with the `X-Woolly-Client` header. The Python package sends "Python app" by default, or the `app` you pass to `Woolly()`.
- `woolly doctor` says whether Claude Code loads your profile at startup.

## 0.6.0

- `woolly connect` adds Woolly to Claude Code, Claude Desktop, Cursor, VS Code, Windsurf, Codex, Gemini CLI, LM Studio and opencode in one step. It keeps the rest of each config and saves a backup first. `woolly disconnect` undoes it.
- `woolly service install` runs Woolly in the background and starts it at login, like Ollama. It uses a LaunchAgent on macOS, a systemd user service on Linux and a startup entry on Windows, none of which need admin rights.
- `woolly serve --proxy` runs the API, review page and proxy in one process. `--log` writes output to a file.
- `woolly doctor` shows the background service and which apps are connected.
- `woolly update` updates Woolly to the latest version and verifies the download. It's the only command that contacts the internet, and only when you run it.
- The website shows the GitHub star count in the nav once it passes 100.

## 0.5.0

- New Python package: `pip install woolly-ai`. Save, recall and add memories to prompts from your own apps. See [Python](https://usewoolly.com/docs/python).
- New README and docs introduction.

## 0.4.0

- `woolly proxy` gives memory to Ollama and to apps that use the OpenAI or Anthropic API. It adds relevant memories to each chat and saves facts you state.
- Windows install script: `irm https://usewoolly.com/install.ps1 | iex`.
- Homebrew: `brew install rjcuff/tap/woolly`.

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

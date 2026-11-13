# Demo of shared project memory: woolly init, one agent saves a decision,
# another agent recalls it. Run from the repo root with VHS
# (https://github.com/charmbracelet/vhs), after make build:
#
#   PATH="$PWD/bin:$PATH" vhs scripts/demo/init.tape
#
# The agents are stand-ins that call Woolly's MCP tools the way Claude Code
# and Cursor do (scripts/demo/agent.sh).

Output docs/init-demo.gif

Set Shell "bash"
Set FontSize 18
Set Width 1200
Set Height 720
Set Padding 24
Set TypingSpeed 40ms
Set Theme "Builtin Light"

Hide
Type `export DEMO="$PWD/scripts/demo" WOOLLY_HOME="$(mktemp -d)" TEAMMATE="$(mktemp -d)" PS1='$ '` Enter
Type `cd "$(mktemp -d)" && mkdir acme && cd acme && git init -q` Enter
Type `claude-code() { "$DEMO/agent.sh" "Claude Code" "$@"; }; cursor() { WOOLLY_HOME="$TEAMMATE" "$DEMO/agent.sh" Cursor "$@"; }` Enter
Type "clear" Enter
Show

Type "woolly init" Sleep 400ms Enter
Sleep 2.5s

Type "# In Claude Code, you pick a database" Enter
Sleep 600ms
Type `claude-code decide "Use SQLite for the database instead of Postgres" "one file, no server"` Sleep 400ms Enter
Sleep 2.5s

Type "sed -n '/## Decisions/,/## Gotchas/p' .woolly/woolly.md" Sleep 400ms Enter
Sleep 3s

Type "# Later, a teammate's Cursor asks" Enter
Sleep 600ms
Type `cursor recall "which database do we use"` Sleep 400ms Enter
Sleep 4s

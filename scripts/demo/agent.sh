#!/bin/sh
# Stands in for an AI app in the demo: calls one Woolly MCP tool the way
# Claude Code or Cursor would, and prints the reply.
#
#   agent.sh "Claude Code" decide "Use SQLite" "one file, no server"
#   agent.sh Cursor recall "which database do we use"
set -e
app=$1 tool=$2
shift 2
args=$(python3 -c '
import json, sys
tool, rest = sys.argv[1], sys.argv[2:]
if tool == "decide":
    print(json.dumps({"decision": rest[0], "why": rest[1] if len(rest) > 1 else ""}))
elif tool == "recall":
    print(json.dumps({"query": rest[0]}))
else:
    print(json.dumps({"content": rest[0]}))
' "$tool" "$@")
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"%s","version":"1"}}}\n' "$app"
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"%s","arguments":%s}}\n' "$tool" "$args"
} | woolly mcp | python3 -c '
import json, sys
for line in sys.stdin:
    r = json.loads(line)
    if r.get("id") == 2:
        for c in r.get("result", {}).get("content", []):
            text = c.get("text", "")
            print("\n".join(l for l in text.splitlines() if not l.startswith("Stored notes")).strip())
'

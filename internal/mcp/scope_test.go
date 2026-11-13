package mcp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/recall"
	"github.com/rjcuff/woolly/internal/share"
)

// Two teammates' agents in the same repo: one saves, the other recalls.
func TestProjectMemoryAcrossAgents(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	os.MkdirAll(filepath.Join(dir, ".git"), 0o755)
	r := share.Find(dir)
	open := func() *Server {
		st, err := memory.Open(filepath.Join(t.TempDir(), "memories"))
		if err != nil {
			t.Fatal(err)
		}
		return &Server{Store: st, Engine: recall.New(st, nil, ""), Version: "test", Repo: r, Project: "acme"}
	}
	claude, cursor := open(), open()
	if _, err := r.Init(claude.Store, share.Options{}); err != nil {
		t.Fatal(err)
	}

	call := func(s *Server, args string) string {
		in := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":` + args + `}}` + "\n"
		var out bytes.Buffer
		s.Serve(context.Background(), strings.NewReader(in), &out)
		return out.String()
	}

	got := call(claude, `"remember","arguments":{"content":"The API runs on port 8080"}`)
	if !strings.Contains(got, "shared with the team") {
		t.Fatalf("auto should pick project for a code fact: %s", got)
	}
	got = call(claude, `"remember","arguments":{"content":"I prefer short answers"}`)
	if strings.Contains(got, "shared with the team") {
		t.Fatalf("a personal fact stays personal: %s", got)
	}
	got = call(claude, `"remember","arguments":{"content":"Lives in Denver","scope":"project"}`)
	if !strings.Contains(got, "never goes in shared project memory") {
		t.Fatalf("personal facts are refused for the repo: %s", got)
	}
	call(claude, `"decide","arguments":{"decision":"Use SQLite","why":"one file"}`)

	file, _ := os.ReadFile(r.File())
	if !strings.Contains(string(file), "port 8080") || !strings.Contains(string(file), "Decided: Use SQLite") || strings.Contains(string(file), "short answers") || strings.Contains(string(file), "Denver") {
		t.Fatalf("woolly.md:\n%s", file)
	}

	got = call(cursor, `"recall","arguments":{"query":"what port does the API use","scope":"project"}`)
	if !strings.Contains(got, "port 8080") {
		t.Fatalf("the other agent should find it: %s", got)
	}
	got = call(cursor, `"recall","arguments":{"query":"short answers","scope":"project"}`)
	if strings.Contains(got, "short answers") {
		t.Fatalf("personal memory never reaches a teammate: %s", got)
	}
}

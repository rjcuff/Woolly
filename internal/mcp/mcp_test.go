package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rjcuff/wooly/internal/memory"
	"github.com/rjcuff/wooly/internal/recall"
)

func TestHandshakeRememberRecall(t *testing.T) {
	store, err := memory.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: store, Engine: recall.New(store, nil, ""), Version: "test"}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"remember","arguments":{"content":"Prefers concise answers","key":"user.style","source":"user"}}}`,
		`{"jsonrpc":"2.0","id":"four","method":"tools/call","params":{"name":"recall","arguments":{"query":"concise"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"nope"}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := s.Serve(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 { // the notification gets no reply
		t.Fatalf("expected 5 responses, got %d:\n%s", len(lines), out.String())
	}
	var resps []map[string]any
	for _, l := range lines {
		var r map[string]any
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatal(err)
		}
		resps = append(resps, r)
	}
	if v := resps[0]["result"].(map[string]any)["protocolVersion"]; v != "2025-06-18" {
		t.Fatalf("protocolVersion = %v", v)
	}
	if n := len(resps[1]["result"].(map[string]any)["tools"].([]any)); n != 6 {
		t.Fatalf("tools = %d", n)
	}
	if !strings.Contains(lines[3], "Prefers concise answers") || resps[3]["id"] != "four" {
		t.Fatalf("recall response: %s", lines[3])
	}
	if resps[4]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Fatalf("expected method-not-found: %s", lines[4])
	}
}

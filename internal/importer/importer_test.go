package importer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const chatgptExport = `[{
  "title": "Moving",
  "mapping": {
    "a": {"message": {"author": {"role": "user"}, "content": {"content_type": "text", "parts": ["I live in Austin. Can you suggest some coffee shops?"]}, "create_time": 1700000000}},
    "b": {"message": {"author": {"role": "assistant"}, "content": {"content_type": "text", "parts": ["I live in a data center."]}, "create_time": 1700000001}},
    "c": {"message": {"author": {"role": "user"}, "content": {"content_type": "text", "parts": ["Quick update: I moved to Denver! Remember that my dog is named Biscuit."]}, "create_time": 1750000000}},
    "d": {"message": {"author": {"role": "user"}, "content": {"content_type": "text", "parts": ["I prefer this. What does it do?"]}, "create_time": 1750000001}},
    "e": null
  }
}]`

const claudeExport = `[{
  "name": "Setup",
  "chat_messages": [
    {"sender": "human", "text": "My name is Ryan. I prefer tabs over spaces.", "created_at": "2026-01-02T10:00:00Z"},
    {"sender": "assistant", "text": "I prefer spaces.", "created_at": "2026-01-02T10:00:05Z"},
    {"sender": "human", "text": "", "content": [{"type": "text", "text": "I work at Acme. Here is code: ` + "```go\\nfunc main() {}\\n```" + `"}], "created_at": "2026-01-03T10:00:00Z"},
    {"sender": "human", "text": "I work as a product designer", "created_at": "2026-01-04T10:00:00Z"}
  ]
}]`

func write(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func contents(cs []Candidate) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Key+"="+c.Content)
	}
	return out
}

func TestChatGPT(t *testing.T) {
	msgs, err := Load("chatgpt", write(t, "conversations.json", chatgptExport))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(contents(Extract(msgs)), "\n")
	want := "user.location=I live in Austin\nuser.location=I moved to Denver\n=My dog is named Biscuit"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestClaude(t *testing.T) {
	msgs, err := Load("claude", write(t, "conversations.json", claudeExport))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(contents(Extract(msgs)), "\n")
	// The Acme message holds code, so it is skipped entirely.
	want := "user.name=My name is Ryan\n=I prefer tabs over spaces\nuser.role=I work as a product designer"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestZip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "export.zip")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("export/conversations.json")
	w.Write([]byte(claudeExport))
	zw.Close()
	f.Close()
	msgs, err := Load("claude", p)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("msgs=%d err=%v", len(msgs), err)
	}
}

func TestUnknownProvider(t *testing.T) {
	if _, err := Load("gemini", write(t, "c.json", "[]")); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitsClausesAndKeysRememberedFacts(t *testing.T) {
	got := strings.Join(contents(Extract([]Message{
		{Text: "I live in Austin and I prefer short answers. Btw, remember that I'm allergic to peanuts!"},
	})), "\n")
	want := "user.location=I live in Austin\n=I prefer short answers\nhealth.allergies=I'm allergic to peanuts"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

package decision

import (
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/memory"
)

func TestFormat(t *testing.T) {
	got := Format("Use in-memory rate limiting.", "one server is enough", []string{"Redis: overkill for one server", " "})
	want := "Decided: Use in-memory rate limiting. Why: one server is enough. Rejected: Redis: overkill for one server."
	if got != want {
		t.Fatalf("got %q", got)
	}
	if Format("Use Go", "", nil) != "Decided: Use Go." {
		t.Fatal("no why or rejected")
	}
}

func TestSection(t *testing.T) {
	mems := []memory.Memory{
		{ID: "a", Content: "Decided: Use Postgres.", Tags: []string{Tag}, Project: "acme", Status: memory.StatusActive},
		{ID: "b", Content: "Decided: Use SQLite.", Tags: []string{Tag}, Project: "notes", Status: memory.StatusActive},
		{ID: "c", Content: "Decided: Tabs not spaces.", Tags: []string{Tag}, Status: memory.StatusActive},
		{ID: "d", Content: "Likes tea", Status: memory.StatusActive},
	}
	s, ids := Section(mems, "acme")
	if len(ids) != 2 || !strings.Contains(s, "Postgres") || strings.Contains(s, "SQLite") || !strings.Contains(s, "Already decided") {
		t.Fatalf("%v\n%s", ids, s)
	}
	if s, _ := Section(nil, "acme"); s != "" {
		t.Fatal("empty")
	}
}

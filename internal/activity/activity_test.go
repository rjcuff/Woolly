package activity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndRecent(t *testing.T) {
	home := t.TempDir()
	l := Open(home)
	l.Add(Entry{App: "Cursor", Action: "remember", IDs: []string{"a"}})
	l.Add(Entry{App: "Claude Code", Action: "recall", Query: "where do I live", IDs: []string{"a", "b"}})
	l.Add(Entry{App: "Cursor", Action: "forget", IDs: []string{"a"}})

	all, err := l.Recent(10, "")
	if err != nil || len(all) != 3 || all[0].Action != "forget" || all[2].Action != "remember" {
		t.Fatalf("newest first: %+v %v", all, err)
	}
	cursor, _ := l.Recent(10, "cursor")
	if len(cursor) != 2 {
		t.Fatalf("filter by app: %+v", cursor)
	}
	one, _ := l.Recent(1, "")
	if len(one) != 1 {
		t.Fatal("limit not applied")
	}
	st, _ := os.Stat(filepath.Join(home, "activity.log"))
	if st.Mode().Perm() != 0o600 && os.PathSeparator == '/' {
		t.Fatalf("log should be private, got %v", st.Mode().Perm())
	}
}

func TestNilLogIsSafe(t *testing.T) {
	var l *Log
	l.Add(Entry{Action: "recall"})
	if got, err := l.Recent(5, ""); got != nil || err != nil {
		t.Fatal("nil log should do nothing")
	}
}

func TestLongQueriesAreTrimmed(t *testing.T) {
	l := Open(t.TempDir())
	l.Add(Entry{App: "x", Action: "recall", Query: strings.Repeat("a", 500)})
	got, _ := l.Recent(1, "")
	if len(got[0].Query) != 200 {
		t.Fatalf("query length %d", len(got[0].Query))
	}
}

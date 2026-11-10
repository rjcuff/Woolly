package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 30, 0, 0, time.UTC)
	m := &Memory{ID: "x1", Content: "Lives in Denver.\n\nLoves hiking.", Key: "user.location",
		Tags: []string{"home", "travel"}, Source: SourceAgent, Status: StatusConflict,
		CreatedAt: now, UpdatedAt: now, ConflictsWith: "y2"}
	got, err := decode(encode(m))
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != m.Content || got.Key != m.Key || got.Source != m.Source || got.Status != m.Status ||
		got.ConflictsWith != "y2" || !got.CreatedAt.Equal(now) || strings.Join(got.Tags, ",") != "home,travel" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestSameKeySupersedes(t *testing.T) {
	s := newStore(t)
	old, _ := s.Remember(RememberInput{Content: "Lives in Austin", Key: "User Location"})
	res, err := s.Remember(RememberInput{Content: "Moved to Denver", Key: "user_location"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Superseded) != 1 || res.Superseded[0].ID != old.Memory.ID {
		t.Fatalf("expected old memory to be superseded, got %+v", res)
	}
	active, _ := s.List(false)
	if len(active) != 1 || active[0].Content != "Moved to Denver" {
		t.Fatalf("active = %+v", active)
	}
	prev, _ := s.Get(old.Memory.ID)
	if prev.Status != StatusSuperseded || prev.SupersededBy != res.Memory.ID {
		t.Fatalf("old memory not marked superseded: %+v", prev)
	}
}

func TestAgentCannotOverrideUser(t *testing.T) {
	s := newStore(t)
	user, _ := s.Remember(RememberInput{Content: "Prefers tabs", Key: "code.indent", Source: SourceUser})
	res, err := s.Remember(RememberInput{Content: "Prefers spaces", Key: "code.indent", Source: SourceAgent})
	if err != nil {
		t.Fatal(err)
	}
	if res.Memory.Status != StatusConflict || res.ConflictsWith == nil || res.ConflictsWith.ID != user.Memory.ID {
		t.Fatalf("expected conflict, got %+v", res)
	}
	active, _ := s.List(false)
	if len(active) != 1 || active[0].Content != "Prefers tabs" {
		t.Fatalf("user fact should stay active, got %+v", active)
	}

	// The user picks the agent's version.
	kept, replaced, err := s.Resolve(res.Memory.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Status != StatusActive || len(replaced) != 1 {
		t.Fatalf("resolve: kept=%+v replaced=%+v", kept, replaced)
	}
	active, _ = s.List(false)
	if len(active) != 1 || active[0].Content != "Prefers spaces" {
		t.Fatalf("after resolve active = %+v", active)
	}
}

func TestUserConfirmingAgentGuessResolvesConflict(t *testing.T) {
	s := newStore(t)
	s.Remember(RememberInput{Content: "Uses Postgres", Key: "project.db", Source: SourceUser})
	s.Remember(RememberInput{Content: "Uses SQLite", Key: "project.db", Source: SourceAgent})
	res, _ := s.Remember(RememberInput{Content: "uses sqlite", Key: "project.db", Source: SourceUser})
	if !res.Duplicate {
		t.Fatalf("expected duplicate, got %+v", res)
	}
	active, _ := s.List(false)
	if len(active) != 1 || active[0].Content != "Uses SQLite" || active[0].Source != SourceUser {
		t.Fatalf("active = %+v", active)
	}
}

func TestDuplicateIsNotStoredTwice(t *testing.T) {
	s := newStore(t)
	s.Remember(RememberInput{Content: "Likes  coffee"})
	res, _ := s.Remember(RememberInput{Content: "likes coffee", Tags: []string{"food"}})
	all, _ := s.List(true)
	if !res.Duplicate || len(all) != 1 || strings.Join(all[0].Tags, ",") != "food" {
		t.Fatalf("dup=%v all=%+v", res.Duplicate, all)
	}
}

func TestPicksUpHandEditsAndForget(t *testing.T) {
	s := newStore(t)
	res, _ := s.Remember(RememberInput{Content: "Original"})
	path := filepath.Join(s.Dir(), res.Memory.ID+".md")
	data, _ := os.ReadFile(path)
	edited := strings.Replace(string(data), "Original", "Edited by hand", 1)
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := s.Get(res.Memory.ID[:12])
	if err != nil || m.Content != "Edited by hand" {
		t.Fatalf("got %+v, %v", m, err)
	}
	if _, err := s.Forget(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
}

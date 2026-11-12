package chats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveListGetDelete(t *testing.T) {
	home := t.TempDir()
	s := Open(home)
	c := &Chat{ID: NewID(), Messages: []Message{{Role: "user", Content: "What should I cook tonight with what's in my fridge?"}, {Role: "assistant", Content: "Pasta"}}}
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	if c.Title != "What should I cook tonight with what's in my fridge?" {
		t.Fatalf("title %q", c.Title)
	}
	list, _ := s.List()
	if len(list) != 1 || list[0].ID != c.ID {
		t.Fatalf("list %+v", list)
	}
	got, err := s.Get(c.ID)
	if err != nil || len(got.Messages) != 2 {
		t.Fatalf("get %+v %v", got, err)
	}
	st, _ := os.Stat(filepath.Join(home, "chats", c.ID+".json"))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	s.Delete(c.ID)
	if _, err := s.Get(c.ID); err != ErrNotFound {
		t.Fatal("should be gone")
	}
}

func TestBadIDs(t *testing.T) {
	s := Open(t.TempDir())
	for _, id := range []string{"../x", "A", "", "a/b"} {
		if _, err := s.Get(id); err != ErrNotFound {
			t.Errorf("%q: %v", id, err)
		}
		if err := s.Save(&Chat{ID: id}); err == nil {
			t.Errorf("%q should not save", id)
		}
	}
}

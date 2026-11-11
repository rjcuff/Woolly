package peer

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/memory"
)

type device struct {
	store *memory.Store
	cfg   *Config
	addr  string
}

func newDevice(t *testing.T, cfg *Config) *device {
	t.Helper()
	home := t.TempDir()
	store, err := memory.Open(filepath.Join(home, "memories"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := Handler(store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &device{store: store, cfg: cfg, addr: strings.TrimPrefix(srv.URL, "http://")}
}

func contents(t *testing.T, s *memory.Store) []string {
	t.Helper()
	mems, _ := s.List(false)
	var out []string
	for _, m := range mems {
		out = append(out, m.Content)
	}
	return out
}

func TestSyncBothWays(t *testing.T) {
	cfg, _ := NewConfig()
	a, b := newDevice(t, cfg), newDevice(t, cfg)
	a.store.Remember(memory.RememberInput{Content: "Lives in Denver"})
	b.store.Remember(memory.RememberInput{Content: "Likes tea"})

	c := &Client{}
	res, err := c.Sync(context.Background(), a.store, cfg, b.addr)
	if err != nil || res.Received != 1 || res.Sent != 1 {
		t.Fatalf("first sync: %+v %v", res, err)
	}
	if len(contents(t, a.store)) != 2 || len(contents(t, b.store)) != 2 {
		t.Fatalf("a %v, b %v", contents(t, a.store), contents(t, b.store))
	}
	if res, _ := c.Sync(context.Background(), a.store, cfg, b.addr); res.Received+res.Sent != 0 {
		t.Fatalf("second sync should do nothing: %+v", res)
	}
}

func TestSyncForgetAndDelete(t *testing.T) {
	cfg, _ := NewConfig()
	a, b := newDevice(t, cfg), newDevice(t, cfg)
	res, _ := a.store.Remember(memory.RememberInput{Content: "Old fact"})
	c := &Client{}
	c.Sync(context.Background(), a.store, cfg, b.addr)

	a.store.Forget(res.Memory.ID)
	c.Sync(context.Background(), a.store, cfg, b.addr)
	if got := contents(t, b.store); len(got) != 0 {
		t.Fatalf("forget should reach b: %v", got)
	}
	if trash, _ := b.store.Trash(); len(trash) != 1 {
		t.Fatal("b should have it in the trash")
	}

	b.store.EmptyTrash()
	c.Sync(context.Background(), a.store, cfg, b.addr)
	if trash, _ := a.store.Trash(); len(trash) != 0 {
		t.Fatal("deleting for good on b should delete on a")
	}
	c.Sync(context.Background(), a.store, cfg, b.addr)
	if trash, _ := b.store.Trash(); len(trash) != 0 {
		t.Fatal("deleted memory came back")
	}
}

func TestSyncNewestEditWins(t *testing.T) {
	cfg, _ := NewConfig()
	a, b := newDevice(t, cfg), newDevice(t, cfg)
	res, _ := a.store.Remember(memory.RememberInput{Content: "Draft"})
	c := &Client{}
	c.Sync(context.Background(), a.store, cfg, b.addr)
	text := "Final"
	b.store.Update(res.Memory.ID, memory.UpdateInput{Content: &text})
	c.Sync(context.Background(), a.store, cfg, b.addr)
	if got := contents(t, a.store); len(got) != 1 || got[0] != "Final" {
		t.Fatalf("a: %v", got)
	}
}

func TestWrongKeyIsRefused(t *testing.T) {
	cfg, _ := NewConfig()
	other, _ := NewConfig()
	a, b := newDevice(t, other), newDevice(t, cfg)
	a.store.Remember(memory.RememberInput{Content: "Secret plans"})
	_, err := (&Client{}).Sync(context.Background(), a.store, other, b.addr)
	if err == nil || !strings.Contains(err.Error(), "join code") {
		t.Fatalf("expected a refusal, got %v", err)
	}
	if len(contents(t, b.store)) != 0 {
		t.Fatal("nothing should reach b")
	}
}

func TestSealedMessagesCantBeSwapped(t *testing.T) {
	key := make([]byte, 32)
	box, _ := seal(key, "reply", map[string]string{"a": "b"})
	var v map[string]string
	if err := open(key, "request", box, &v); err == nil {
		t.Fatal("a reply must not open as a request")
	}
	box[len(box)-1] ^= 1
	if err := open(key, "reply", box, &v); err == nil {
		t.Fatal("a changed message must not open")
	}
}

func TestBadIDsAreRejected(t *testing.T) {
	for _, id := range []string{"../etc/passwd", ".hidden", "a/b", "", "x..y"} {
		if memory.ValidID(id) {
			t.Errorf("%q should be invalid", id)
		}
	}
}

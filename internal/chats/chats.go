// Package chats saves conversations from the Woolly chat as JSON files.
package chats

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Message is one turn. Saved lists facts Woolly learned from it.
type Message struct {
	Role     string   `json:"role"`
	Content  string   `json:"content"`
	Memories []string `json:"memories,omitempty"` // IDs recalled for this reply
	Saved    []Saved  `json:"saved,omitempty"`
}

// Saved is a fact learned from a message.
type Saved struct {
	ID       string `json:"id"`
	Fact     string `json:"fact"`
	Action   string `json:"action"`             // add, update or conflict
	Replaces string `json:"replaces,omitempty"` // memory it replaced, for undo
}

// Chat is one conversation.
type Chat struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Model     string    `json:"model,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

// Summary is a chat without its messages.
type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ErrNotFound means there's no chat with that ID.
var ErrNotFound = errors.New("chat not found")

var validID = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// Store keeps chats in a folder.
type Store struct{ dir string }

// Open returns the store in home/chats.
func Open(home string) *Store { return &Store{dir: filepath.Join(home, "chats")} }

// NewID returns a new chat ID.
func NewID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

func (s *Store) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, id+".json"), nil
}

// List returns chats, newest first.
func (s *Store) List() ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Summary{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c, err := s.Get(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, Summary{ID: c.ID, Title: c.Title, UpdatedAt: c.UpdatedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get loads a chat.
func (s *Store) Get(id string) (*Chat, error) {
	p, err := s.path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var c Chat
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes a chat, readable only by you. It fills in the title and times.
func (s *Store) Save(c *Chat) error {
	p, err := s.path(c.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if c.Title == "" {
		for _, m := range c.Messages {
			if m.Role == "user" {
				c.Title = title(m.Content)
				break
			}
		}
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Delete removes a chat.
func (s *Store) Delete(id string) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func title(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 60 {
		s = string(r[:59]) + "…"
	}
	return s
}

package memory

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// What device sync needs from the store.

// Item describes one memory for sync.
type Item struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	Trash     bool      `json:"trash,omitempty"`
	Sum       string    `json:"sum"` // SHA-256 of the file
}

// Newer reports whether a should replace b: the later change wins, and on a
// tie the larger checksum, so every device picks the same one.
func (a Item) Newer(b Item) bool {
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.After(b.UpdatedAt)
	}
	return a.Sum > b.Sum
}

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// ValidID reports whether id is safe to use as a file name.
func ValidID(id string) bool { return validID.MatchString(id) && !strings.Contains(id, "..") }

func sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Manifest lists every memory, current and in the trash.
func (s *Store) Manifest() ([]Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Item
	for _, place := range []struct {
		dir   string
		trash bool
	}{{s.dir, false}, {s.trashDir(), true}} {
		entries, err := os.ReadDir(place.dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !isMemoryFile(e.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(place.dir, e.Name()))
			if err != nil {
				continue
			}
			m, err := decode(data)
			if err != nil || !ValidID(m.ID) {
				continue
			}
			out = append(out, Item{ID: m.ID, UpdatedAt: m.UpdatedAt, Trash: place.trash, Sum: sum(data)})
		}
	}
	return out, nil
}

// Raw returns a memory's file as stored, and whether it's in the trash.
func (s *Store) Raw(id string) ([]byte, bool, error) {
	if !ValidID(id) {
		return nil, false, ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.mems[id]; ok && m.file != "" {
		data, err := os.ReadFile(m.file)
		return data, false, err
	}
	if data, err := os.ReadFile(filepath.Join(s.dir, id+".md")); err == nil {
		return data, false, nil
	}
	if data, err := os.ReadFile(filepath.Join(s.trashDir(), id+".md")); err == nil {
		return data, true, nil
	}
	return nil, false, ErrNotFound
}

// Apply saves a memory from another device if it's newer than ours.
func (s *Store) Apply(data []byte, trash bool) (bool, error) {
	m, err := decode(data)
	if err != nil {
		return false, fmt.Errorf("bad memory from another device: %w", err)
	}
	if !ValidID(m.ID) {
		return false, fmt.Errorf("bad memory id %q from another device", m.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isTombstone(m.ID) {
		return false, nil
	}
	in := Item{ID: m.ID, UpdatedAt: m.UpdatedAt, Trash: trash, Sum: sum(data)}
	live, dead := filepath.Join(s.dir, m.ID+".md"), filepath.Join(s.trashDir(), m.ID+".md")
	if old, ok := s.mems[m.ID]; ok && old.file != "" {
		live = old.file
	}
	for _, p := range []struct {
		path  string
		trash bool
	}{{live, false}, {dead, true}} {
		cur, err := os.ReadFile(p.path)
		if err != nil {
			continue
		}
		old, err := decode(cur)
		if err != nil {
			continue
		}
		if !in.Newer(Item{ID: m.ID, UpdatedAt: old.UpdatedAt, Trash: p.trash, Sum: sum(cur)}) {
			return false, nil
		}
	}
	dst, other := live, dead
	if trash {
		dst, other = dead, live
		if err := os.MkdirAll(s.trashDir(), 0o700); err != nil {
			return false, err
		}
	}
	if err := writeFile(dst, data); err != nil {
		return false, err
	}
	if err := os.Remove(other); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, s.load()
}

// Delete removes a memory for good because another device deleted it.
func (s *Store) Delete(id string) error {
	if !ValidID(id) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addTombstone(id)
	for _, p := range []string{filepath.Join(s.dir, id+".md"), filepath.Join(s.trashDir(), id+".md")} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if m, ok := s.mems[id]; ok && m.file != "" {
		_ = os.Remove(m.file)
	}
	return s.load()
}

// Tombstones lists memories deleted for good.
func (s *Store) Tombstones() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readTombstones()
}

func (s *Store) tombstonePath() string { return filepath.Join(filepath.Dir(s.dir), "deleted") }

func (s *Store) readTombstones() []string {
	f, err := os.Open(s.tombstonePath())
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if id := strings.TrimSpace(sc.Text()); ValidID(id) {
			out = append(out, id)
		}
	}
	return out
}

func (s *Store) isTombstone(id string) bool {
	for _, t := range s.readTombstones() {
		if t == id {
			return true
		}
	}
	return false
}

func (s *Store) addTombstone(id string) {
	if !ValidID(id) || s.isTombstone(id) {
		return
	}
	f, err := os.OpenFile(s.tombstonePath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, id)
}

// writeFile replaces path atomically with a private file.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound is returned when no memory matches an ID.
var ErrNotFound = errors.New("memory not found")

// Store keeps memories as Markdown files in one directory. It caches them in
// memory and reloads automatically when files change on disk, so a running
// server picks up edits made by the CLI, an editor, or a sync tool.
type Store struct {
	dir  string
	mu   sync.Mutex
	mems map[string]*Memory
	fp   string // fingerprint of the directory at last load
}

// Open opens (creating if needed) a store rooted at dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}
	s := &Store{dir: dir}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Dir returns the directory holding the memory files.
func (s *Store) Dir() string { return s.dir }

func isMemoryFile(name string) bool {
	return strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, ".")
}

func (s *Store) fingerprint() (string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return "", err
	}
	var n int
	var latest time.Time
	for _, e := range entries {
		if e.IsDir() || !isMemoryFile(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		n++
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return fmt.Sprintf("%d:%d", n, latest.UnixNano()), nil
}

func (s *Store) load() error {
	fp, err := s.fingerprint()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	mems := make(map[string]*Memory, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isMemoryFile(e.Name()) {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		m, err := decode(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "woolly: skipping %s: %v\n", e.Name(), err)
			continue
		}
		m.file = path
		mems[m.ID] = m
	}
	s.mems, s.fp = mems, fp
	return nil
}

// refresh reloads from disk if anything changed. Caller holds s.mu.
func (s *Store) refresh() error {
	fp, err := s.fingerprint()
	if err != nil {
		return err
	}
	if fp == s.fp && s.mems != nil {
		return nil
	}
	return s.load()
}

// write saves a memory atomically (temp file + rename). Caller holds s.mu.
func (s *Store) write(m *Memory) error {
	if m.file == "" {
		m.file = filepath.Join(s.dir, m.ID+".md")
	}
	tmp, err := os.CreateTemp(s.dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(encode(m)); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), m.file); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	s.mems[m.ID] = m
	return nil
}

// find resolves a full ID or an unambiguous ID prefix. Caller holds s.mu.
func (s *Store) find(id string) (*Memory, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrNotFound
	}
	if m, ok := s.mems[id]; ok {
		return m, nil
	}
	var match *Memory
	for mid, m := range s.mems {
		if strings.HasPrefix(mid, id) {
			if match != nil {
				return nil, fmt.Errorf("id prefix %q matches more than one memory", id)
			}
			match = m
		}
	}
	if match == nil {
		return nil, ErrNotFound
	}
	return match, nil
}

// RememberInput describes a new memory.
type RememberInput struct {
	Content string
	Key     string   // optional stable slot, e.g. "user.location"
	Tags    []string // optional
	Source  Source   // defaults to SourceUser
}

// RememberResult reports what Remember did.
type RememberResult struct {
	Memory        Memory   `json:"memory"`
	Duplicate     bool     `json:"duplicate,omitempty"`
	Superseded    []Memory `json:"superseded,omitempty"`
	ConflictsWith *Memory  `json:"conflicts_with,omitempty"`
}

// Remember stores a new memory and applies Woolly's freshness rules:
//
//   - The same content again is a duplicate; the existing memory is refreshed.
//   - A memory with the same key as existing ones supersedes them, so recall
//     returns the newest fact instead of a stale one.
//   - Unless it comes from a less trusted source than the current fact (an
//     agent guess vs. something the user said). Then it is stored as a
//     conflict for the user to review, and the trusted fact stays active.
func (s *Store) Remember(in RememberInput) (*RememberResult, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, errors.New("memory content is empty")
	}
	src := in.Source
	if src == "" {
		src = SourceUser
	}
	if src.Rank() == 0 {
		return nil, fmt.Errorf("unknown source %q (use user, import or agent)", src)
	}
	key := NormalizeKey(in.Key)
	tags := NormalizeTags(in.Tags)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	// Duplicate: same key and same text as a live memory.
	for _, ex := range s.mems {
		if ex.Status == StatusSuperseded || ex.Key != key || !sameText(ex.Content, content) {
			continue
		}
		ex.UpdatedAt = now
		ex.Tags = mergeTags(ex.Tags, tags)
		if src.Rank() > ex.Source.Rank() {
			ex.Source = src // the user confirmed something an agent guessed
		}
		if err := s.write(ex); err != nil {
			return nil, err
		}
		if ex.Status == StatusConflict {
			if other, ok := s.mems[ex.ConflictsWith]; !ok || ex.Source.Rank() >= other.Source.Rank() {
				if _, err := s.resolveLocked(ex, now); err != nil {
					return nil, err
				}
			}
		}
		return &RememberResult{Memory: ex.clone(), Duplicate: true}, nil
	}

	m := &Memory{
		ID: newID(now), Content: content, Key: key, Tags: tags,
		Source: src, Status: StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	res := &RememberResult{}

	if key != "" {
		var same []*Memory
		var stronger *Memory
		for _, ex := range s.mems {
			if ex.Key != key || ex.Status == StatusSuperseded {
				continue
			}
			same = append(same, ex)
			if ex.Status == StatusActive && ex.Source.Rank() > src.Rank() &&
				(stronger == nil || ex.UpdatedAt.After(stronger.UpdatedAt)) {
				stronger = ex
			}
		}
		if stronger != nil {
			m.Status = StatusConflict
			m.ConflictsWith = stronger.ID
			c := stronger.clone()
			res.ConflictsWith = &c
		} else {
			for _, ex := range same {
				ex.Status = StatusSuperseded
				ex.SupersededBy = m.ID
				ex.ConflictsWith = ""
				ex.UpdatedAt = now
				if err := s.write(ex); err != nil {
					return nil, err
				}
				res.Superseded = append(res.Superseded, ex.clone())
			}
		}
	}

	if err := s.write(m); err != nil {
		return nil, err
	}
	res.Memory = m.clone()
	return res, nil
}

// Get returns a memory by ID or unambiguous ID prefix.
func (s *Store) Get(id string) (Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return Memory{}, err
	}
	m, err := s.find(id)
	if err != nil {
		return Memory{}, err
	}
	return m.clone(), nil
}

// List returns memories, newest first. Without includeAll it returns only
// active memories.
func (s *Store) List(includeAll bool) ([]Memory, error) {
	return s.filter(func(m *Memory) bool { return includeAll || m.Status == StatusActive })
}

// Conflicts returns memories waiting for the user to resolve them.
func (s *Store) Conflicts() ([]Memory, error) {
	return s.filter(func(m *Memory) bool { return m.Status == StatusConflict })
}

func (s *Store) filter(keep func(*Memory) bool) ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return nil, err
	}
	var out []Memory
	for _, m := range s.mems {
		if keep(m) {
			out = append(out, m.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

// Forget permanently deletes a memory's file.
func (s *Store) Forget(id string) (Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return Memory{}, err
	}
	m, err := s.find(id)
	if err != nil {
		return Memory{}, err
	}
	if err := os.Remove(m.file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Memory{}, err
	}
	delete(s.mems, m.ID)
	return m.clone(), nil
}

// Resolve makes the given memory the current fact for its key and marks every
// other live memory with that key as superseded by it.
func (s *Store) Resolve(id string) (Memory, []Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return Memory{}, nil, err
	}
	keep, err := s.find(id)
	if err != nil {
		return Memory{}, nil, err
	}
	replaced, err := s.resolveLocked(keep, time.Now().UTC())
	if err != nil {
		return Memory{}, nil, err
	}
	return keep.clone(), replaced, nil
}

func (s *Store) resolveLocked(keep *Memory, now time.Time) ([]Memory, error) {
	var replaced []Memory
	if keep.Key != "" {
		for _, ex := range s.mems {
			if ex.ID == keep.ID || ex.Key != keep.Key || ex.Status == StatusSuperseded {
				continue
			}
			ex.Status = StatusSuperseded
			ex.SupersededBy = keep.ID
			ex.ConflictsWith = ""
			ex.UpdatedAt = now
			if err := s.write(ex); err != nil {
				return nil, err
			}
			replaced = append(replaced, ex.clone())
		}
	}
	keep.Status = StatusActive
	keep.ConflictsWith = ""
	keep.SupersededBy = ""
	keep.UpdatedAt = now
	if err := s.write(keep); err != nil {
		return nil, err
	}
	return replaced, nil
}

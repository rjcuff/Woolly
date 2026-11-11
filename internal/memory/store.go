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

	"github.com/rjcuff/woolly/internal/secrets"
)

// ErrNotFound is returned when no memory matches an ID.
var ErrNotFound = errors.New("memory not found")

// SensitiveError is returned when a memory looks like it contains a secret.
type SensitiveError struct {
	What string // e.g. "an OpenAI API key"
}

func (e *SensitiveError) Error() string {
	return "not saved: this looks like it contains " + e.What + ". Woolly doesn't store secrets"
}

// InjectionError is returned when an AI or an import tries to save text that
// reads like instructions to an AI rather than a fact about the user.
type InjectionError struct{}

func (e *InjectionError) Error() string {
	return "not saved: this reads like instructions to an AI, not a fact about the user"
}

func checkSensitive(text string, allow bool) error {
	if allow {
		return nil
	}
	if found := secrets.Scan(text); len(found) > 0 {
		return &SensitiveError{What: secrets.Describe(found)}
	}
	return nil
}

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
	// AllowSensitive saves the memory even if it looks like it holds a secret.
	AllowSensitive bool
	// Via names the app saving the memory.
	Via string
	// Project scopes the memory to one project. Keys only clash within the
	// same project.
	Project string
}

// RememberResult reports what Remember did.
type RememberResult struct {
	Memory        Memory   `json:"memory"`
	Duplicate     bool     `json:"duplicate,omitempty"`
	Superseded    []Memory `json:"superseded,omitempty"`
	ConflictsWith *Memory  `json:"conflicts_with,omitempty"`
}

// Remember saves a memory. A repeat refreshes the existing one. A newer fact
// with the same key replaces older ones, unless it comes from a less trusted
// source, in which case it's saved as a conflict for the user to review.
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
	if err := checkSensitive(content, in.AllowSensitive); err != nil {
		return nil, err
	}
	// Anything you type yourself is trusted. Text from an AI or an import
	// could have come from a malicious web page or document.
	if src != SourceUser && secrets.LooksLikeInstructions(content) {
		return nil, &InjectionError{}
	}
	key := NormalizeKey(in.Key)
	tags := NormalizeTags(in.Tags)
	project := NormalizeProject(in.Project)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	// Duplicate: same key and same text as a live memory.
	for _, ex := range s.mems {
		if ex.Status == StatusSuperseded || ex.Key != key || ex.Project != project || !sameText(ex.Content, content) {
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
		Via: CleanApp(in.Via), Project: project,
	}
	res := &RememberResult{}

	if key != "" {
		var same []*Memory
		var stronger *Memory
		for _, ex := range s.mems {
			if ex.Key != key || ex.Project != project || ex.Status == StatusSuperseded {
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

// UpdateInput changes an existing memory. Nil fields are left alone.
type UpdateInput struct {
	Content        *string
	Key            *string
	Tags           *[]string
	Project        *string
	AllowSensitive bool
}

// Update edits a memory in place. If the memory is current and has a key,
// it becomes the current fact for that key.
func (s *Store) Update(id string, in UpdateInput) (Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return Memory{}, err
	}
	m, err := s.find(id)
	if err != nil {
		return Memory{}, err
	}
	if in.Content != nil {
		c := strings.TrimSpace(*in.Content)
		if c == "" {
			return Memory{}, errors.New("memory content is empty")
		}
		if err := checkSensitive(c, in.AllowSensitive); err != nil {
			return Memory{}, err
		}
		m.Content = c
	}
	if in.Key != nil {
		m.Key = NormalizeKey(*in.Key)
	}
	if in.Tags != nil {
		m.Tags = NormalizeTags(*in.Tags)
	}
	if in.Project != nil {
		m.Project = NormalizeProject(*in.Project)
	}
	m.UpdatedAt = time.Now().UTC()
	if m.Status == StatusActive && m.Key != "" {
		if _, err := s.resolveLocked(m, m.UpdatedAt); err != nil {
			return Memory{}, err
		}
	} else if err := s.write(m); err != nil {
		return Memory{}, err
	}
	return m.clone(), nil
}

// trashDir holds forgotten memories until the trash is emptied.
func (s *Store) trashDir() string { return filepath.Join(filepath.Dir(s.dir), "trash") }

// Forget moves a memory to the trash, where Restore can bring it back.
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
	if err := os.MkdirAll(s.trashDir(), 0o700); err != nil {
		return Memory{}, err
	}
	// The new time lets other devices see that forgetting is the newest change.
	m.UpdatedAt = time.Now().UTC()
	if err := writeFile(filepath.Join(s.trashDir(), m.ID+".md"), encode(m)); err != nil {
		return Memory{}, err
	}
	if err := os.Remove(m.file); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Memory{}, err
	}
	delete(s.mems, m.ID)
	return m.clone(), nil
}

// Trash lists forgotten memories, newest first.
func (s *Store) Trash() ([]Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.trashDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Memory
	for _, e := range entries {
		if e.IsDir() || !isMemoryFile(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.trashDir(), e.Name()))
		if err != nil {
			continue
		}
		if m, err := decode(data); err == nil {
			out = append(out, m.clone())
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Restore brings a forgotten memory back from the trash.
func (s *Store) Restore(id string) (Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trash := s.trashDir()
	entries, err := os.ReadDir(trash)
	if err != nil {
		return Memory{}, ErrNotFound
	}
	var match string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".md")
		if name == id || (id != "" && strings.HasPrefix(name, id)) {
			if match != "" && name != id {
				return Memory{}, fmt.Errorf("id prefix %q matches more than one memory", id)
			}
			match = e.Name()
		}
	}
	if match == "" {
		return Memory{}, ErrNotFound
	}
	src := filepath.Join(trash, match)
	data, err := os.ReadFile(src)
	if err != nil {
		return Memory{}, err
	}
	m, err := decode(data)
	if err != nil {
		return Memory{}, err
	}
	m.UpdatedAt = time.Now().UTC()
	if err := writeFile(filepath.Join(s.dir, match), encode(m)); err != nil {
		return Memory{}, err
	}
	if err := os.Remove(src); err != nil {
		return Memory{}, err
	}
	if err := s.load(); err != nil {
		return Memory{}, err
	}
	got, ok := s.mems[strings.TrimSuffix(match, ".md")]
	if !ok {
		return Memory{}, ErrNotFound
	}
	return got.clone(), nil
}

// EmptyTrash permanently deletes everything in the trash.
func (s *Store) EmptyTrash() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.trashDir())
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || !isMemoryFile(e.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(s.trashDir(), e.Name())); err == nil {
			n++
			s.addTombstone(strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return n, nil
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
			if ex.ID == keep.ID || ex.Key != keep.Key || ex.Project != keep.Project || ex.Status == StatusSuperseded {
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

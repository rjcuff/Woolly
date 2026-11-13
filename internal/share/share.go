// Package share keeps a project's memory in its git repo, in
// .woolly/woolly.md, so everyone's agents working on the repo read and
// update the same notes. Personal memories never go there.
package share

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/secrets"
)

// handoffKey matches handoff.Key. Handoffs are personal and never shared.
const handoffKey = "handoff"

// attributes keeps both sides when two people add lines at once. Woolly
// sorts out duplicates the next time it reads the file.
const attributes = ".woolly/woolly.md merge=union"

// Repo is a project folder that may keep its memory in .woolly/.
type Repo struct {
	Dir     string // the folder holding .woolly/, or the git repo's root
	Project string // the project name
}

// Find returns the project holding dir: the nearest folder with .woolly/,
// or else the git repo. It returns nil outside both.
func Find(dir string) *Repo {
	home, _ := os.UserHomeDir()
	for d := dir; d != ""; {
		if filepath.Clean(d) == filepath.Clean(home) {
			return nil
		}
		r := &Repo{Dir: d, Project: memory.NormalizeProject(filepath.Base(d))}
		if r.Enabled() {
			if p := r.Config().Project; p != "" {
				r.Project = p
			}
			return r
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return r
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil
		}
		d = parent
	}
	return nil
}

// File is the path of woolly.md.
func (r *Repo) File() string { return filepath.Join(r.Dir, ".woolly", "woolly.md") }

// legacy is where Woolly 0.10 to 0.12 kept shared notes, one file each.
func (r *Repo) legacy() string { return filepath.Join(r.Dir, ".woolly", "memories") }

// Enabled reports whether the project keeps its memory in the repo.
func (r *Repo) Enabled() bool {
	if _, err := os.Stat(r.File()); err == nil {
		return true
	}
	st, err := os.Stat(r.legacy())
	return err == nil && st.IsDir()
}

// Config reads the settings at the top of woolly.md.
func (r *Repo) Config() Config {
	data, err := os.ReadFile(r.File())
	if err != nil {
		return Config{Project: r.Project, Capture: true}
	}
	c := parseDoc(string(data)).cfg
	if c.Project == "" {
		c.Project = r.Project
	}
	return c
}

// Options changes settings when you run init.
type Options struct {
	Export  []string // "claude", "agents"; nil leaves it as is
	Capture *bool
}

// Init creates .woolly/woolly.md, or updates its settings, and syncs.
func (r *Repo) Init(store *memory.Store, opts Options) (Result, error) {
	if err := os.MkdirAll(filepath.Dir(r.File()), 0o755); err != nil {
		return Result{}, err
	}
	d := newDoc(r.Project)
	if data, err := os.ReadFile(r.File()); err == nil {
		d = parseDoc(string(data))
		if d.cfg.Project == "" {
			d.cfg.Project = r.Project
		}
	}
	if opts.Export != nil {
		d.cfg.Export = opts.Export
	}
	if opts.Capture != nil {
		d.cfg.Capture = *opts.Capture
	}
	if err := os.WriteFile(r.File(), []byte(d.render()), 0o644); err != nil {
		return Result{}, err
	}
	if err := r.addAttributes(); err != nil {
		return Result{}, err
	}
	return r.Sync(store)
}

func (r *Repo) addAttributes() error {
	path := filepath.Join(r.Dir, ".gitattributes")
	cur, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if strings.Contains(string(cur), attributes) {
		return nil
	}
	text := string(cur)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+attributes+"\n"), 0o644)
}

// Result says what a sync did.
type Result struct {
	Imported, Exported, Removed, Skipped int
}

// state is what this computer saw in woolly.md at the last sync, so it can
// tell a line a teammate deleted from a note that's new here.
type state struct {
	SyncedAt time.Time `json:"synced_at"`
	IDs      []string  `json:"ids"`
}

func statePath(store *memory.Store) string {
	return filepath.Join(filepath.Dir(store.Dir()), "repos.json")
}

func loadStates(store *memory.Store) map[string]state {
	out := map[string]state{}
	if data, err := os.ReadFile(statePath(store)); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

// Sync brings woolly.md and this computer's memories for the project up to
// date with each other. A change made here since the last sync wins over the
// file; otherwise the file wins, since it's what the team committed.
func (r *Repo) Sync(store *memory.Store) (Result, error) {
	var res Result
	if !r.Enabled() {
		return res, nil
	}
	if err := r.migrate(store, &res); err != nil {
		return res, err
	}
	data, err := os.ReadFile(r.File())
	d := newDoc(r.Project)
	if err == nil {
		d = parseDoc(string(data))
		if d.cfg.Project == "" {
			d.cfg.Project = r.Project
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return res, err
	}

	states := loadStates(store)
	st := states[r.Dir]
	had := map[string]bool{}
	for _, id := range st.IDs {
		had[id] = true
	}

	all, err := store.List(true)
	if err != nil {
		return res, err
	}
	local := map[string]memory.Memory{}
	for _, m := range all {
		if m.Project == r.Project {
			local[suffix(m.ID)] = m
		}
	}
	trashed := map[string]memory.Memory{}
	if trash, err := store.Trash(); err == nil {
		for _, m := range trash {
			if m.Project == r.Project {
				trashed[suffix(m.ID)] = m
			}
		}
	}

	inFile := map[string]bool{} // notes the file keeps
	held := map[string]bool{}   // lines kept as written, not from this computer
	var kept, fresh []line
	for _, ln := range d.lines {
		if ln.raw != "" {
			kept = append(kept, ln)
			continue
		}
		if inFile[ln.id] || held[ln.id] {
			ln.id = "" // the same line twice, after a merge
		}
		// A repo is shared input: anything holding a secret or reading like
		// instructions to an AI stays in the file but isn't loaded.
		if len(secrets.Scan(ln.text)) > 0 || secrets.LooksLikeInstructions(ln.text) {
			res.Skipped++
			if ln.id != "" {
				held[ln.id] = true
			}
			ln.raw = ln.String()
			kept = append(kept, ln)
			continue
		}
		m, isLocal := local[ln.id]
		t, isTrashed := trashed[ln.id]
		switch {
		case ln.id != "" && isLocal && m.Status != memory.StatusSuperseded:
			inFile[ln.id] = true
			if m.UpdatedAt.After(st.SyncedAt) {
				continue // changed here since the last sync
			}
			if err := update(store, m, ln); err != nil {
				return res, err
			} else if changed(m, ln) {
				res.Imported++
			}
		case ln.id != "" && isLocal:
			res.Removed++ // replaced by a newer fact here
		case ln.id != "" && isTrashed && t.UpdatedAt.After(st.SyncedAt):
			res.Removed++ // forgotten here
		case ln.id != "" && isTrashed:
			// Someone put it back.
			back, err := store.Restore(t.ID)
			if err != nil {
				return res, err
			}
			if err := update(store, back, ln); err != nil {
				return res, err
			}
			inFile[ln.id] = true
			res.Imported++
		case ln.id != "" && had[ln.id]:
			res.Removed++ // deleted for good here
		default:
			fresh = append(fresh, ln) // a teammate's note, or a line someone typed
		}
	}

	// Notes deleted from the file go first, so a teammate's replacement
	// doesn't show up as a conflict with the note it replaced.
	for _, m := range all {
		id := suffix(m.ID)
		if m.Project != r.Project || m.Status == memory.StatusSuperseded || inFile[id] || held[id] || !had[id] || m.UpdatedAt.After(st.SyncedAt) {
			continue
		}
		if _, err := store.Forget(m.ID); err != nil {
			return res, err
		}
		res.Removed++
	}

	for _, ln := range fresh {
		id := ""
		if ln.id != "" {
			id = time.Now().UTC().Format("20060102-150405") + "-" + ln.id
		}
		var tags []string
		if tag := TagFor(ln.section); tag != "" {
			tags = []string{tag}
		}
		// Notes from the repo rank like an agent's, so anyone's newer note
		// for the same key replaces them, and a fact you stated yourself
		// is flagged as a conflict instead of being overwritten.
		saved, err := store.Remember(memory.RememberInput{
			ID: id, Content: ln.text, Key: ln.key, Tags: tags,
			Source: memory.SourceAgent, Via: ln.by, Project: r.Project,
		})
		if err != nil {
			res.Skipped++
			ln.raw = ln.String()
			kept = append(kept, ln)
			continue
		}
		inFile[suffix(saved.Memory.ID)] = true
		res.Imported++
	}

	// Write out this computer's notes for the project.
	all, err = store.List(true)
	if err != nil {
		return res, err
	}
	lines := kept
	var ids []string
	for id := range held {
		ids = append(ids, id)
	}
	for _, m := range all {
		if m.Project != r.Project || m.Status == memory.StatusSuperseded || m.Key == handoffKey || strings.HasPrefix(m.Key, "user.") {
			continue
		}
		id := suffix(m.ID)
		if held[id] {
			continue
		}
		if !inFile[id] {
			res.Exported++
		}
		lines = append(lines, lineFor(m))
		ids = append(ids, id)
	}
	d.lines = lines
	if out := d.render(); out != string(data) {
		if err := os.WriteFile(r.File(), []byte(out), 0o644); err != nil {
			return res, err
		}
	}

	states[r.Dir] = state{SyncedAt: time.Now().UTC(), IDs: ids}
	if b, err := json.MarshalIndent(states, "", "  "); err == nil {
		_ = os.WriteFile(statePath(store), b, 0o600)
	}
	return res, r.export(d)
}

func lineFor(m memory.Memory) line {
	return line{section: SectionOf(m), text: oneLine(m.Content), id: suffix(m.ID), key: m.Key, by: m.Via}
}

func changed(m memory.Memory, ln line) bool {
	return oneLine(m.Content) != ln.text || m.Key != ln.key || SectionOf(m) != ln.section
}

// update makes a memory match a line someone edited.
func update(store *memory.Store, m memory.Memory, ln line) error {
	if !changed(m, ln) {
		return nil
	}
	in := memory.UpdateInput{}
	if oneLine(m.Content) != ln.text {
		in.Content = &ln.text
	}
	if m.Key != ln.key {
		in.Key = &ln.key
	}
	if SectionOf(m) != ln.section {
		tags := withSection(m.Tags, ln.section)
		in.Tags = &tags
	}
	_, err := store.Update(m.ID, in)
	return err
}

// migrate moves notes from the old one-file-per-note folder into woolly.md.
func (r *Repo) migrate(store *memory.Store, res *Result) error {
	dir := r.legacy()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		m, err := memory.ParseFile(data)
		if err != nil || !memory.ValidID(m.ID) || e.Name() != m.ID+".md" ||
			m.Project != r.Project || m.Key == handoffKey ||
			len(secrets.Scan(m.Content)) > 0 || secrets.LooksLikeInstructions(m.Content) {
			res.Skipped++
			continue
		}
		if ok, err := store.Apply(data, false); err != nil {
			return err
		} else if ok {
			res.Imported++
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(r.Dir, ".woolly", "README.md"))
	if _, err := os.Stat(r.File()); errors.Is(err, os.ErrNotExist) {
		d := newDoc(r.Project)
		if hasSection(filepath.Join(r.Dir, "AGENTS.md")) {
			d.cfg.Export = []string{"agents"} // it was kept up to date before
		}
		if err := os.WriteFile(r.File(), []byte(d.render()), 0o644); err != nil {
			return err
		}
		return r.addAttributes()
	}
	return nil
}

// Disable stops keeping memory in the repo: it removes .woolly/ and Woolly's
// sections in CLAUDE.md, AGENTS.md and .gitattributes. Memories on this
// computer stay.
func (r *Repo) Disable() error {
	if err := os.RemoveAll(filepath.Join(r.Dir, ".woolly")); err != nil {
		return err
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := removeSection(filepath.Join(r.Dir, name)); err != nil {
			return err
		}
	}
	path := filepath.Join(r.Dir, ".gitattributes")
	if cur, err := os.ReadFile(path); err == nil {
		out := strings.Replace(string(cur), attributes+"\n", "", 1)
		if strings.TrimSpace(out) == "" {
			return os.Remove(path)
		}
		return os.WriteFile(path, []byte(out), 0o644)
	}
	return nil
}

// Describe sums up a sync in a line, or "" when nothing changed.
func (res Result) Describe() string {
	var parts []string
	if res.Imported > 0 {
		parts = append(parts, fmt.Sprintf("%d from the repo", res.Imported))
	}
	if res.Exported > 0 {
		parts = append(parts, fmt.Sprintf("%d written to it", res.Exported))
	}
	if res.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", res.Removed))
	}
	if res.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped as unsafe", res.Skipped))
	}
	return strings.Join(parts, ", ")
}

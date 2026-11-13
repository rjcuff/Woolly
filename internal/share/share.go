// Package share keeps a project's memories in its git repo, in .woolly/, so
// everyone's agents working on the repo share the same project notes.
package share

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/secrets"
)

// handoffKey matches handoff.Key. Handoffs are personal and never shared.
const handoffKey = "handoff"

const (
	agentsStart = "<!-- woolly:start -->"
	agentsEnd   = "<!-- woolly:end -->"
	maxAgents   = 25
)

const readme = `# Woolly project memory

These are notes about this project that AI coding tools saved with Woolly
(https://usewoolly.com): its stack, commands and decisions. Each file is one
memory. Commit them so everyone's agents start with the same context.

Woolly only writes notes about this project here, never anything personal.
`

// Repo is a git repo whose project memories can be shared.
type Repo struct {
	Dir     string // the repo's root folder
	Project string // the project name, from the folder name
}

// Find returns the git repo holding dir, or nil outside one.
func Find(dir string) *Repo {
	home, _ := os.UserHomeDir()
	for d := dir; d != ""; {
		if filepath.Clean(d) == filepath.Clean(home) {
			return nil
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return &Repo{Dir: d, Project: memory.NormalizeProject(filepath.Base(d))}
		}
		parent := filepath.Dir(d)
		if parent == d {
			return nil
		}
		d = parent
	}
	return nil
}

func (r *Repo) folder() string { return filepath.Join(r.Dir, ".woolly", "memories") }

// Enabled reports whether the repo shares its project memories.
func (r *Repo) Enabled() bool {
	st, err := os.Stat(r.folder())
	return err == nil && st.IsDir()
}

// Enable starts sharing: it creates .woolly/memories and a short README.
func (r *Repo) Enable() error {
	if err := os.MkdirAll(r.folder(), 0o755); err != nil {
		return err
	}
	p := filepath.Join(r.Dir, ".woolly", "README.md")
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(p, []byte(readme), 0o644)
	}
	return nil
}

// Result says what a sync did.
type Result struct {
	Imported, Exported, Removed, Skipped int
}

// Sync brings the repo's folder and this computer's memories for the project
// up to date with each other. The newer copy of each memory wins.
func (r *Repo) Sync(store *memory.Store) (Result, error) {
	var res Result
	if !r.Enabled() {
		return res, nil
	}
	dir := r.folder()

	// Bring in teammates' notes. A repo is shared input, so anything that
	// isn't a plain project note, holds a secret or reads like instructions
	// to an AI is skipped.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return res, err
	}
	inRepo := map[string]*memory.Memory{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		m, err := memory.ParseFile(data)
		if err != nil || !memory.ValidID(m.ID) || e.Name() != m.ID+".md" {
			res.Skipped++
			continue
		}
		if m.Project != r.Project || m.Key == handoffKey || len(secrets.Scan(m.Content)) > 0 || secrets.LooksLikeInstructions(m.Content) {
			res.Skipped++
			continue
		}
		inRepo[m.ID] = m
		changed, err := store.Apply(data, false)
		if err != nil {
			return res, err
		}
		if changed {
			res.Imported++
		}
	}

	// Write out this computer's notes for the project.
	items, err := store.Manifest()
	if err != nil {
		return res, err
	}
	for _, it := range items {
		data, trash, err := store.Raw(it.ID)
		if err != nil {
			continue
		}
		m, err := memory.ParseFile(data)
		if err != nil || m.Project != r.Project || m.Key == handoffKey {
			continue
		}
		path := filepath.Join(dir, m.ID+".md")
		old := inRepo[m.ID]
		if trash {
			if old != nil && !old.UpdatedAt.After(m.UpdatedAt) {
				if os.Remove(path) == nil {
					res.Removed++
				}
			}
			continue
		}
		if old != nil && !m.UpdatedAt.After(old.UpdatedAt) {
			continue
		}
		if cur, err := os.ReadFile(path); err == nil && bytes.Equal(cur, data) {
			continue
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return res, err
		}
		res.Exported++
	}
	return res, nil
}

// WriteAgentsMD keeps a short list of the project's notes in AGENTS.md,
// between Woolly's markers, so tools without MCP read them too. It only adds
// the section to an existing AGENTS.md, or creates the file, when create is
// true; otherwise it updates a section that's already there.
func (r *Repo) WriteAgentsMD(store *memory.Store, create bool) (bool, error) {
	path := filepath.Join(r.Dir, "AGENTS.md")
	cur, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	text := string(cur)
	start, end := strings.Index(text, agentsStart), strings.Index(text, agentsEnd)
	if (start < 0 || end < start) && !create {
		return false, nil
	}

	mems, err := store.List(false)
	if err != nil {
		return false, err
	}
	var lines []string
	for _, m := range mems {
		if m.Project == r.Project && m.Key != handoffKey && len(lines) < maxAgents {
			lines = append(lines, "- "+strings.Join(strings.Fields(m.Content), " "))
		}
	}
	section := fmt.Sprintf("%s\n## Project notes\n\nKept up to date by Woolly from what AI tools learned about this project.\n\n%s\n%s",
		agentsStart, strings.Join(lines, "\n"), agentsEnd)
	if len(lines) == 0 {
		section = fmt.Sprintf("%s\n## Project notes\n\nNone yet. Woolly adds notes here as AI tools learn about this project.\n%s", agentsStart, agentsEnd)
	}

	var out string
	switch {
	case start >= 0 && end > start:
		out = text[:start] + section + text[end+len(agentsEnd):]
	case strings.TrimSpace(text) == "":
		out = "# AGENTS.md\n\n" + section + "\n"
	default:
		out = strings.TrimRight(text, "\n") + "\n\n" + section + "\n"
	}
	if out == text {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(out), 0o644)
}

// Disable stops sharing: it removes .woolly/ and the AGENTS.md section.
// Memories on this computer stay.
func (r *Repo) Disable() error {
	if err := os.RemoveAll(filepath.Join(r.Dir, ".woolly")); err != nil {
		return err
	}
	path := filepath.Join(r.Dir, "AGENTS.md")
	cur, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(cur)
	start, end := strings.Index(text, agentsStart), strings.Index(text, agentsEnd)
	if start < 0 || end < start {
		return nil
	}
	out := strings.TrimRight(text[:start], "\n") + "\n" + strings.TrimLeft(text[end+len(agentsEnd):], "\n")
	if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "# AGENTS.md")) == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

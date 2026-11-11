// Package memory defines Woolly's memory model and its on-disk store.
//
// Every memory is a plain Markdown file with a small front-matter header, so
// users can read, edit, grep, sync and version-control their memories with
// ordinary tools. The files are the source of truth; any index is derived.
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// Source records where a memory came from. It decides which memory wins when
// two memories about the same key disagree.
type Source string

const (
	SourceUser   Source = "user"   // the user stated it directly
	SourceImport Source = "import" // imported from another tool's export
	SourceAgent  Source = "agent"  // an AI inferred it
)

// Rank orders sources by trust: user > import > agent. Unknown sources rank 0.
func (s Source) Rank() int {
	switch s {
	case SourceUser:
		return 3
	case SourceImport:
		return 2
	case SourceAgent:
		return 1
	}
	return 0
}

// ParseSource parses a source name. The empty string is not a valid source.
func ParseSource(s string) (Source, bool) {
	src := Source(strings.ToLower(strings.TrimSpace(s)))
	if src.Rank() == 0 {
		return "", false
	}
	return src, true
}

// Status is the lifecycle state of a memory.
type Status string

const (
	StatusActive     Status = "active"     // current and used for recall
	StatusSuperseded Status = "superseded" // replaced by a newer memory with the same key
	StatusConflict   Status = "conflict"   // disagrees with a more trusted memory; needs review
)

// Memory is a single remembered fact.
type Memory struct {
	ID            string    `json:"id"`
	Content       string    `json:"content"`
	Key           string    `json:"key,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	Source        Source    `json:"source"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	SupersededBy  string    `json:"superseded_by,omitempty"`
	ConflictsWith string    `json:"conflicts_with,omitempty"`
	// Via is the app that saved the memory, like "Cursor" or "command line".
	Via string `json:"via,omitempty"`
	// Project is set for facts that only matter in one project, like a
	// repo's database. Facts about the user have no project.
	Project string `json:"project,omitempty"`

	file string // path on disk; empty until saved
}

func (m *Memory) clone() Memory {
	c := *m
	c.Tags = append([]string(nil), m.Tags...)
	return c
}

// NormalizeKey lowercases a key and joins whitespace with underscores, so
// "User Location" and "user_location" are the same key.
func NormalizeKey(k string) string {
	return strings.Join(strings.Fields(strings.ToLower(k)), "_")
}

// NormalizeTags trims, lowercases, de-duplicates and sorts tags.
func NormalizeTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		for _, part := range strings.Split(t, ",") {
			p := strings.Join(strings.Fields(strings.ToLower(part)), "-")
			if p == "" || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func mergeTags(a, b []string) []string {
	return NormalizeTags(append(append([]string(nil), a...), b...))
}

func sameText(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}

// newID returns a time-sortable, filename-safe ID such as 20261110-153000-1a2b3c4d.
func newID(t time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return t.UTC().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

// NormalizeProject lowercases a project name and keeps it filename and
// URL safe: "My App" becomes "my-app".
func NormalizeProject(p string) string {
	p = strings.Join(strings.Fields(strings.ToLower(p)), "-")
	var b strings.Builder
	for _, r := range p {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-.")
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// In reports whether the memory belongs in project: global memories are in
// every project, and project memories only in their own. An empty project
// means "everywhere", which includes every memory.
func (m *Memory) In(project string) bool {
	return project == "" || m.Project == "" || m.Project == project
}

// CleanApp tidies an app name from a client: one line, at most 40 characters.
func CleanApp(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 40 {
		s = string(r[:40])
	}
	return s
}

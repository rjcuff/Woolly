package memory

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// encode renders a memory as Markdown with a front-matter header:
//
//	---
//	id: 20261110-153000-1a2b3c4d
//	key: user.location
//	tags: home, travel
//	source: user
//	status: active
//	created_at: 2026-11-10T15:30:00Z
//	updated_at: 2026-11-10T15:30:00Z
//	---
//
//	Lives in Denver, Colorado.
func encode(m *Memory) []byte {
	var b strings.Builder
	w := func(k, v string) {
		v = strings.TrimSpace(strings.ReplaceAll(v, "\n", " "))
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	b.WriteString("---\n")
	w("id", m.ID)
	w("key", m.Key)
	w("tags", strings.Join(m.Tags, ", "))
	w("source", string(m.Source))
	w("status", string(m.Status))
	w("created_at", m.CreatedAt.UTC().Format(time.RFC3339Nano))
	w("updated_at", m.UpdatedAt.UTC().Format(time.RFC3339Nano))
	w("superseded_by", m.SupersededBy)
	w("conflicts_with", m.ConflictsWith)
	w("via", m.Via)
	w("project", m.Project)
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(m.Content))
	b.WriteString("\n")
	return []byte(b.String())
}

// decode parses a memory file. It is forgiving about hand edits: unknown
// header fields are ignored and missing status/source get sensible defaults.
func decode(data []byte) (*Memory, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, errors.New("missing front matter (file must start with ---)")
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, errors.New("unterminated front matter")
	}
	header, body := rest[:end], rest[end+len("\n---"):]

	m := &Memory{Content: strings.TrimSpace(body)}
	for _, line := range strings.Split(header, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "id":
			m.ID = v
		case "key":
			m.Key = NormalizeKey(v)
		case "tags":
			m.Tags = NormalizeTags([]string{v})
		case "source":
			m.Source = Source(strings.ToLower(v))
		case "status":
			m.Status = Status(strings.ToLower(v))
		case "created_at":
			m.CreatedAt, _ = time.Parse(time.RFC3339Nano, v)
		case "updated_at":
			m.UpdatedAt, _ = time.Parse(time.RFC3339Nano, v)
		case "superseded_by":
			m.SupersededBy = v
		case "conflicts_with":
			m.ConflictsWith = v
		case "via":
			m.Via = v
		case "project":
			m.Project = NormalizeProject(v)
		}
	}
	if m.ID == "" {
		return nil, errors.New("missing id")
	}
	if m.Source.Rank() == 0 {
		m.Source = SourceUser
	}
	switch m.Status {
	case StatusActive, StatusSuperseded, StatusConflict:
	default:
		m.Status = StatusActive
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = m.CreatedAt
	}
	return m, nil
}

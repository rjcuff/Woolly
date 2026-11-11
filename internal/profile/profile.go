// Package profile summarizes everything currently true about the user, in a
// form that can be pasted into any AI's custom instructions.
package profile

import (
	"regexp"
	"sort"
	"strings"

	"github.com/rjcuff/woolly/internal/memory"
)

// Profile groups current memories into sections.
type Profile struct {
	Facts       []memory.Memory `json:"facts"`
	Preferences []memory.Memory `json:"preferences"`
	Notes       []memory.Memory `json:"notes"`
}

var preference = regexp.MustCompile(`(?i)^(?:i\s+)?(?:really\s+)?(?:prefer|like|love|enjoy|dislike|hate|avoid|don't like|do not like|always|never)\b|\bprefers?\b`)

// Build sorts active memories into facts (anything with a key),
// preferences and notes.
func Build(mems []memory.Memory) Profile {
	var p Profile
	for _, m := range mems {
		// Project memories describe a project, not the user.
		if m.Status != memory.StatusActive || m.Project != "" {
			continue
		}
		switch {
		case m.Key != "":
			p.Facts = append(p.Facts, m)
		case hasTag(m, "preference") || preference.MatchString(m.Content):
			p.Preferences = append(p.Preferences, m)
		default:
			p.Notes = append(p.Notes, m)
		}
	}
	sort.SliceStable(p.Facts, func(i, j int) bool { return p.Facts[i].Key < p.Facts[j].Key })
	return p
}

func hasTag(m memory.Memory, tag string) bool {
	for _, t := range m.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Empty reports whether there is nothing to show.
func (p Profile) Empty() bool {
	return len(p.Facts)+len(p.Preferences)+len(p.Notes) == 0
}

// Markdown renders the profile as a short Markdown document.
func (p Profile) Markdown() string {
	if p.Empty() {
		return "Woolly doesn't know anything about you yet.\n"
	}
	var b strings.Builder
	b.WriteString("# About me\n")
	section := func(title string, mems []memory.Memory) {
		if len(mems) == 0 {
			return
		}
		b.WriteString("\n## " + title + "\n\n")
		for _, m := range mems {
			b.WriteString("- " + strings.TrimSuffix(m.Content, ".") + "\n")
		}
	}
	section("Facts", p.Facts)
	section("Preferences", p.Preferences)
	section("Notes", p.Notes)
	return b.String()
}

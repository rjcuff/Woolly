// Package decision stores choices with the reason and the options that were
// rejected, so agents stop suggesting what was already turned down.
package decision

import (
	"fmt"
	"strings"

	"github.com/rjcuff/woolly/internal/gitinfo"
	"github.com/rjcuff/woolly/internal/memory"
)

// Tag marks a memory as a decision.
const Tag = "decision"

// Format writes a decision as one memory.
func Format(choice, why string, rejected []string) string {
	choice = strings.TrimSuffix(strings.TrimSpace(choice), ".")
	var b strings.Builder
	b.WriteString("Decided: " + choice + ".")
	if why = strings.TrimSuffix(strings.TrimSpace(why), "."); why != "" {
		b.WriteString(" Why: " + why + ".")
	}
	var r []string
	for _, x := range rejected {
		if x = strings.TrimSpace(x); x != "" {
			r = append(r, strings.TrimSuffix(x, "."))
		}
	}
	if len(r) > 0 {
		b.WriteString(" Rejected: " + strings.Join(r, "; ") + ".")
	}
	return b.String()
}

// Is reports whether a memory is a decision.
func Is(m memory.Memory) bool {
	for _, t := range m.Tags {
		if t == Tag {
			return true
		}
	}
	return false
}

// Section lists the decisions for a project (and decisions with no
// project) for the start of a session, and returns their IDs.
func Section(mems []memory.Memory, project string) (string, []string) {
	var lines, ids []string
	for _, m := range mems {
		if Is(m) && m.Status == memory.StatusActive && m.In(project) {
			lines = append(lines, "- "+strings.TrimSpace(m.Content))
			ids = append(ids, m.ID)
		}
	}
	if len(lines) == 0 {
		return "", nil
	}
	return "# Already decided\n\nDon't suggest a rejected option again unless the user brings it up.\n\n" + strings.Join(lines, "\n") + "\n\n", ids
}

// Staleness says whether files a project note mentions changed since it
// was saved, like "may be out of date: api/ratelimit.go changed in 3 commits
// since". It returns "" when nothing changed or it can't tell.
func Staleness(repoDir string, m memory.Memory) string {
	if repoDir == "" || m.Commit == "" {
		return ""
	}
	paths := gitinfo.Paths(repoDir, m.Content)
	if len(paths) == 0 {
		return ""
	}
	n := gitinfo.Changes(repoDir, m.Commit, paths)
	if n == 0 {
		return ""
	}
	what := paths[0]
	if len(paths) > 1 {
		what = fmt.Sprintf("%s and %d more", paths[0], len(paths)-1)
	}
	commits := "commits"
	if n == 1 {
		commits = "commit"
	}
	return fmt.Sprintf("may be out of date: %s changed in %d %s since", what, n, commits)
}

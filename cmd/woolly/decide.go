package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rjcuff/woolly/internal/decision"
	"github.com/rjcuff/woolly/internal/gitinfo"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/share"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ", ") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// cmdDecide saves a decision with the reason and what was rejected.
func cmdDecide(args []string) error {
	fs := newFlags("decide")
	why := fs.String("why", "", "the reason")
	var rejected multi
	fs.Var(&rejected, "rejected", `an option that was turned down, like "Redis: overkill" (repeat for more)`)
	key := fs.String("key", "", "a stable name, so a later decision on the same thing replaces this one")
	project := fs.String("project", "", "the project (default: this repo)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	choice := strings.TrimSpace(strings.Join(pos, " "))
	if choice == "" {
		return errors.New(`what was decided? try: woolly decide "Use Postgres" --why "we need JSON queries" --rejected "MySQL: weaker JSON support"`)
	}
	p := *project
	if p == "" {
		if cwd, err := os.Getwd(); err == nil {
			if r := share.Find(cwd); r != nil {
				p = r.Project
			}
		}
	}
	a, err := open()
	if err != nil {
		return err
	}
	res, err := a.store.Remember(memory.RememberInput{
		Content: decision.Format(choice, *why, rejected), Key: *key, Tags: []string{decision.Tag},
		Source: memory.SourceUser, Via: cli, Project: p, Commit: commitFor(p),
	})
	if err != nil {
		return err
	}
	a.record("remember", []string{res.Memory.ID}, 0)
	if p != "" {
		if cwd, err := os.Getwd(); err == nil {
			a.syncRepo(cwd)
		}
	}
	fmt.Println("Saved:", res.Memory.Content)
	if p != "" {
		fmt.Println("Project:", p)
	}
	return nil
}

// commitFor returns this repo's commit when saving a note for its project.
func commitFor(project string) string {
	if project == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	r := share.Find(cwd)
	if r == nil || r.Project != memory.NormalizeProject(project) {
		return ""
	}
	return gitinfo.Head(r.Dir)
}

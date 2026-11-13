package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/share"
)

// cmdInit keeps this repo's project memory in .woolly/woolly.md, so
// teammates' agents read and update the same notes.
func cmdInit(args []string) error {
	fs := newFlags("init")
	export := fs.String("export", "", "keep a Woolly section in these files: claude, agents, or none")
	agents := fs.Bool("agents-md", false, "same as --export agents")
	capture := fs.Bool("capture", false, "save decisions when a Claude Code session ends (the default)")
	noCapture := fs.Bool("no-capture", false, "don't save decisions when a session ends")
	stop := fs.Bool("stop", false, "stop keeping memory in this repo and remove .woolly/")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	r := share.Find(cwd)
	if r == nil {
		return errors.New("run this inside a git repo")
	}
	if *stop {
		if err := r.Disable(); err != nil {
			return err
		}
		fmt.Println("Removed .woolly/ from this repo. Your memories are still on this computer.")
		return nil
	}

	var opts share.Options
	if *agents && *export == "" {
		*export = "agents"
	}
	if *export != "" {
		opts.Export = []string{}
		for _, e := range strings.Split(*export, ",") {
			switch e = strings.ToLower(strings.TrimSpace(e)); e {
			case "claude", "agents":
				opts.Export = append(opts.Export, e)
			case "none", "":
			default:
				return fmt.Errorf("--export takes claude, agents or none, not %q", e)
			}
		}
	}
	if *capture || *noCapture {
		on := !*noCapture
		opts.Capture = &on
	}

	a, err := open()
	if err != nil {
		return err
	}
	first := !r.Enabled()
	res, err := r.Init(a.store, opts)
	if err != nil {
		return err
	}
	cfg := r.Config()
	if first {
		fmt.Printf("Created .woolly/woolly.md for %s", r.Project)
		if res.Exported == 1 {
			fmt.Print(" with 1 note you already had")
		} else if res.Exported > 1 {
			fmt.Printf(" with %d notes you already had", res.Exported)
		}
		fmt.Println(".")
		fmt.Println("Commit .woolly/ and .gitattributes so your team's agents share the same project memory.")
		fmt.Println("Only notes about this project go there, never anything personal.")
	} else if d := res.Describe(); d != "" {
		fmt.Println("Synced: " + d + ".")
	} else {
		fmt.Println("Project memory is up to date.")
	}
	if len(cfg.Export) > 0 {
		fmt.Printf("Keeping a Woolly section in %s.\n", exportFiles(cfg.Export))
	}
	if cfg.Capture {
		fmt.Println("Decisions from Claude Code sessions are saved here when they end (turn off with --no-capture).")
	}
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		if strings.HasPrefix(one, "One") {
			return one
		}
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func exportFiles(list []string) string {
	var out []string
	for _, e := range list {
		out = append(out, map[string]string{"claude": "CLAUDE.md", "agents": "AGENTS.md"}[e])
	}
	return strings.Join(out, " and ")
}

// cmdProject shows this repo's project memory, or opens it to edit.
func cmdProject(args []string) error {
	fs := newFlags("project")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	sub := ""
	if len(pos) > 0 {
		sub = pos[0]
	}
	if sub != "" && sub != "edit" {
		return fmt.Errorf("unknown: woolly project %s (try woolly project or woolly project edit)", sub)
	}
	cwd, _ := os.Getwd()
	r := share.Find(cwd)
	if r == nil || !r.Enabled() {
		return errors.New("no project memory here yet. Run woolly init in your repo")
	}
	a, err := open()
	if err != nil {
		return err
	}
	if _, err := r.Sync(a.store); err != nil {
		return err
	}

	if sub == "edit" {
		editor := os.Getenv("VISUAL")
		if editor == "" {
			editor = os.Getenv("EDITOR")
		}
		if editor == "" {
			editor = "vi"
			if runtime.GOOS == "windows" {
				editor = "notepad"
			}
		}
		parts := strings.Fields(editor)
		cmd := exec.Command(parts[0], append(parts[1:], r.File())...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s didn't finish (%v), so nothing was saved. Set EDITOR to change the editor", parts[0], err)
		}
		res, err := r.Sync(a.store)
		if err != nil {
			return err
		}
		switch {
		case res.Imported+res.Removed == 0 && res.Skipped == 0:
			fmt.Println("No changes.")
		default:
			var done []string
			if res.Imported > 0 {
				done = append(done, plural(res.Imported, "note added or changed", "notes added or changed"))
			}
			if res.Removed > 0 {
				done = append(done, plural(res.Removed, "removed", "removed"))
			}
			if len(done) > 0 {
				fmt.Println("Saved: " + strings.Join(done, ", ") + ".")
			}
			if res.Skipped > 0 {
				fmt.Printf("%s held a secret or read like instructions to an AI, so agents won't see it.\n", plural(res.Skipped, "One line", "lines"))
			}
		}
		return nil
	}

	mems, err := a.store.List(false)
	if err != nil {
		return err
	}
	by := map[string][]memory.Memory{}
	n := 0
	for _, m := range mems {
		if m.Project == r.Project && m.Key != "handoff" {
			s := share.SectionOf(m)
			by[s] = append(by[s], m)
			n++
		}
	}
	fmt.Printf("%s project memory, from .woolly/woolly.md (%d)\n", r.Project, n)
	if n == 0 {
		fmt.Println("\nNothing yet. Agents add notes here as they learn about the project, or run woolly project edit.")
		return nil
	}
	for _, name := range share.SectionNames() {
		if len(by[name]) == 0 {
			continue
		}
		fmt.Printf("\n%s\n", name)
		for _, m := range by[name] {
			via := ""
			if m.Via != "" {
				via = "  (" + m.Via + ")"
			}
			fmt.Printf("  - %s%s\n", m.Content, via)
		}
	}
	fmt.Println("\nEdit with: woolly project edit")
	return nil
}

// syncRepo brings the repo's project memory up to date, if it keeps one.
func (a *app) syncRepo(dir string) {
	if r := share.Find(dir); r != nil && r.Enabled() {
		_, _ = r.Sync(a.store)
	}
}

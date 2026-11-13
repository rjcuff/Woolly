package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/rjcuff/woolly/internal/share"
)

// cmdShare keeps this repo's project memories in .woolly/, so teammates'
// agents share them.
func cmdShare(args []string) error {
	fs := newFlags("share")
	agents := fs.Bool("agents-md", false, "also keep a Project notes section in AGENTS.md")
	stop := fs.Bool("stop", false, "stop sharing and remove .woolly/ from this repo")
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
		fmt.Println("Stopped sharing. Your memories are still on this computer.")
		return nil
	}
	a, err := open()
	if err != nil {
		return err
	}
	first := !r.Enabled()
	if err := r.Enable(); err != nil {
		return err
	}
	res, err := r.Sync(a.store)
	if err != nil {
		return err
	}
	if *agents {
		if _, err := r.WriteAgentsMD(a.store, true); err != nil {
			return err
		}
	} else {
		_, _ = r.WriteAgentsMD(a.store, false)
	}
	if first {
		fmt.Printf("Sharing %s's project memories in .woolly/memories (%d so far).\n", r.Project, res.Exported)
		fmt.Println("Commit .woolly/ so your team's agents start with the same notes.")
		fmt.Println("Only notes about this project are shared, never anything personal.")
		return nil
	}
	fmt.Printf("Synced: %d new from the repo, %d written to it.\n", res.Imported, res.Exported)
	if res.Skipped > 0 {
		fmt.Printf("Skipped %d that weren't plain project notes.\n", res.Skipped)
	}
	return nil
}

// syncRepo shares project memories with the repo at dir, if it shares them.
func (a *app) syncRepo(dir string) {
	r := share.Find(dir)
	if r == nil || !r.Enabled() {
		return
	}
	_, _ = r.Sync(a.store)
	_, _ = r.WriteAgentsMD(a.store, false)
}

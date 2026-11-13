package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/handoff"
	"github.com/rjcuff/woolly/internal/memory"
)

// hookInput is what Claude Code sends a hook on stdin.
type hookInput struct {
	Cwd        string `json:"cwd"`
	Transcript string `json:"transcript_path"`
	Event      string `json:"hook_event_name"`
}

// readHook reads a hook's JSON from stdin, waiting at most a second.
func readHook() hookInput {
	var in hookInput
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice != 0 {
		return in
	}
	done := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		done <- b
	}()
	select {
	case b := <-done:
		_ = json.Unmarshal(b, &in)
	case <-time.After(time.Second):
	}
	return in
}

// cmdHandoff shows where you left off in this project. Run from a Claude
// Code hook, it saves a handoff instead: a quick one right away, then a
// better one from the local model in the background.
func cmdHandoff(args []string) error {
	fs := newFlags("handoff")
	appName := fs.String("app", "", "the app the session was in")
	transcript := fs.String("transcript", "", "summarize this transcript (used by the hook)")
	dir := fs.String("cwd", "", "project folder")
	summarize := fs.Bool("summarize", false, "write the handoff with the local model (used by the hook)")
	clear := fs.Bool("clear", false, "remove the handoff for this project")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	app := activity.AppName(*appName, "")

	if *transcript == "" && !*clear {
		if in := readHook(); in.Transcript != "" {
			// From a hook: never fail loudly, and finish fast.
			_ = saveFromHook(in, app)
			return nil
		}
	}

	cwd := *dir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	project := projectName(cwd)
	a, err := open()
	if err != nil {
		return err
	}

	if *summarize && *transcript != "" {
		turns, err := handoff.ReadTranscript(*transcript)
		if err != nil || len(turns) == 0 {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		text, err := handoff.Summarize(ctx, a.ollama, turns)
		if err != nil {
			return nil
		}
		if m, err := handoff.Save(a.store, project, app, text); err == nil {
			a.log.Add(activity.Entry{App: app, Action: "handoff", IDs: []string{m.ID}})
		}
		return nil
	}

	h := handoff.Latest(a.store, project)
	if *clear {
		if h != nil {
			if _, err := a.store.Forget(h.ID); err != nil {
				return err
			}
		}
		fmt.Println("Cleared.")
		return nil
	}
	if h == nil {
		fmt.Println("No handoff yet for this project. Connect Claude Code (woolly connect claude-code) and one is saved when a session ends or compacts.")
		return nil
	}
	fmt.Printf("Where you left off%s:\n\n%s\n", from(*h), h.Content)
	return nil
}

func saveFromHook(in hookInput, app string) error {
	a, err := open()
	if err != nil {
		return err
	}
	project := projectName(in.Cwd)
	turns, err := handoff.ReadTranscript(in.Transcript)
	if err != nil || len(turns) == 0 {
		return err
	}
	if quick := handoff.Quick(turns); quick != "" {
		if m, err := handoff.Save(a.store, project, app, quick); err == nil {
			a.log.Add(activity.Entry{App: app, Action: "handoff", IDs: []string{m.ID}})
		}
	}
	// The better summary runs on its own, since apps only wait a moment
	// for hooks when a session ends.
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if ok, err := a.ollama.HasModel(ctx, a.ollama.ChatModel); err != nil || !ok {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startDetached(exe, []string{"handoff", "--summarize", "--transcript", in.Transcript, "--cwd", in.Cwd, "--app", app})
}

// from says which app saved a handoff and when.
func from(m memory.Memory) string {
	s := ""
	if m.Via != "" {
		s = " in " + m.Via
	}
	return s + ", " + timeAgo(m.UpdatedAt)
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

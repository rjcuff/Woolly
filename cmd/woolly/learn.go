package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/learn"
)

func (a *app) learner() *learn.Learner {
	return &learn.Learner{Store: a.store, Engine: a.engine, Model: a.ollama}
}

func cmdLearn(args []string) error {
	fs := newFlags("learn")
	project := fs.String("project", "", "project for facts about a codebase")
	dryRun := fs.Bool("dry-run", false, "show what would be saved without saving")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	text := strings.Join(pos, " ")
	if strings.TrimSpace(text) == "" {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if err != nil {
			return err
		}
		text = string(data)
	}
	if strings.TrimSpace(text) == "" {
		return errors.New(`nothing to learn from; try: woolly learn "I just moved to Denver" or pipe in a conversation`)
	}
	a, err := open()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	steps, err := a.learner().Learn(ctx, text, learn.Options{Project: *project, Via: cli, DryRun: *dryRun})
	if err != nil {
		return fmt.Errorf("%v\nwoolly learn needs Ollama with a chat model. Run: ollama pull %s", err, a.ollama.ChatModel)
	}
	for _, s := range steps {
		if s.Memory != nil && s.Action != "skip" {
			a.record("remember", []string{s.Memory.ID}, 0)
		}
	}
	if *asJSON {
		return printJSON(steps)
	}
	if len(steps) == 0 {
		fmt.Println("Nothing worth remembering.")
		return nil
	}
	verbs := map[string]string{"add": "Added", "update": "Updated", "skip": "Already known", "conflict": "Conflict", "refused": "Refused"}
	if *dryRun {
		verbs = map[string]string{"add": "Would add", "update": "Would update", "skip": "Already known", "conflict": "Conflict", "refused": "Refused"}
	}
	for _, s := range steps {
		line := fmt.Sprintf("%-14s %s", verbs[s.Action]+":", s.Fact)
		if s.Project != "" {
			line += " (project: " + s.Project + ")"
		}
		if s.Reason != "" {
			line += "\n               " + s.Reason
		}
		fmt.Println(line)
	}
	return nil
}

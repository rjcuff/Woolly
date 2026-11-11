package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/memory"
)

func TestBuildContext(t *testing.T) {
	mems := []memory.Memory{
		{ID: "a", Content: "Woolly uses Go", Status: memory.StatusActive},
		{ID: "b", Content: "I prefer short answers", Status: memory.StatusActive},
		{ID: "c", Content: "Deploys on Fridays", Tags: []string{"woolly"}, Status: memory.StatusActive},
	}
	out, ids := buildContext(mems, "woolly")
	if len(ids) != 2 || !strings.Contains(out, "# This project (woolly)") || !strings.Contains(out, "I prefer short answers") {
		t.Fatalf("ids %v\n%s", ids, out)
	}
	if strings.Count(out, "Woolly uses Go") != 1 {
		t.Fatalf("project memory listed twice:\n%s", out)
	}
	if !strings.HasPrefix(out, "Notes about the user") {
		t.Fatal("context must be framed as notes")
	}
}

func TestBuildContextLeavesOtherProjectsOut(t *testing.T) {
	mems := []memory.Memory{
		{ID: "a", Content: "Uses Postgres", Project: "acme", Status: memory.StatusActive},
		{ID: "b", Content: "Uses SQLite", Project: "notes", Status: memory.StatusActive},
		{ID: "c", Content: "Likes tea", Status: memory.StatusActive},
	}
	out, ids := buildContext(mems, "acme")
	if len(ids) != 1 || !strings.Contains(out, "Uses Postgres") || !strings.Contains(out, "Likes tea") || strings.Contains(out, "SQLite") {
		t.Fatalf("ids %v\n%s", ids, out)
	}
	if out, _ := buildContext(mems, ""); strings.Contains(out, "Postgres") || strings.Contains(out, "SQLite") {
		t.Fatalf("outside a project, project memories stay out:\n%s", out)
	}
}

func TestBuildContextStaysUnderLimit(t *testing.T) {
	var mems []memory.Memory
	for i := 0; i < 2000; i++ {
		mems = append(mems, memory.Memory{ID: "x", Content: "A fairly long note that repeats many times over", Status: memory.StatusActive})
	}
	out, _ := buildContext(mems, "")
	if len(out) > contextLimit+100 || !strings.Contains(out, "More in Woolly") {
		t.Fatalf("len %d", len(out))
	}
}

func TestProjectName(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "acme")
	os.MkdirAll(filepath.Join(repo, ".git"), 0o755)
	os.MkdirAll(filepath.Join(repo, "web", "src"), 0o755)
	if got := projectName(filepath.Join(repo, "web", "src")); got != "acme" {
		t.Fatalf("got %q", got)
	}
	if h, _ := os.UserHomeDir(); projectName(h) != "" {
		t.Fatal("home folder is not a project")
	}
}

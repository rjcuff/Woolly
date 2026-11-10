package profile

import (
	"testing"

	"github.com/rjcuff/wooly/internal/memory"
)

func TestBuildAndMarkdown(t *testing.T) {
	mems := []memory.Memory{
		{Content: "Moved to Denver.", Key: "user.location", Status: memory.StatusActive},
		{Content: "Lives in Austin", Key: "user.location", Status: memory.StatusSuperseded},
		{Content: "My name is Ryan", Key: "user.name", Status: memory.StatusActive},
		{Content: "I prefer short answers", Status: memory.StatusActive},
		{Content: "Dark mode everywhere", Tags: []string{"preference"}, Status: memory.StatusActive},
		{Content: "Working on Woolly", Status: memory.StatusActive},
		{Content: "Prefers spaces", Key: "code.indent", Status: memory.StatusConflict},
	}
	got := Build(mems).Markdown()
	want := `# About me

## Facts

- Moved to Denver
- My name is Ryan

## Preferences

- I prefer short answers
- Dark mode everywhere

## Notes

- Working on Woolly
`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEmpty(t *testing.T) {
	if got := Build(nil).Markdown(); got != "Woolly doesn't know anything about you yet.\n" {
		t.Fatalf("got %q", got)
	}
}

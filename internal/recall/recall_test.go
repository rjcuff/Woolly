package recall

import (
	"context"
	"errors"
	"testing"

	"github.com/rjcuff/woolly/internal/memory"
)

func seed(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{
		"Prefers dark mode in every editor",
		"Has two dogs named Biscuit and Pepper",
		"Works on a Go project called Woolly",
	} {
		if _, err := s.Remember(memory.RememberInput{Content: c}); err != nil {
			t.Fatal(err)
		}
	}
	s.Remember(memory.RememberInput{Content: "Lives in Austin", Key: "user.location"})
	s.Remember(memory.RememberInput{Content: "Lives in Denver now", Key: "user.location"})
	return s
}

func TestKeywordRecall(t *testing.T) {
	e := New(seed(t), nil, "")
	res, mode, err := e.Recall(context.Background(), "what are my dog's names", 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeKeyword || len(res) == 0 || res[0].Memory.Content != "Has two dogs named Biscuit and Pepper" {
		t.Fatalf("mode=%s res=%+v", mode, res)
	}
}

func TestStaleFactsAreHidden(t *testing.T) {
	e := New(seed(t), nil, "")
	res, _, _ := e.Recall(context.Background(), "where does the user live", 5, false)
	if len(res) != 1 || res[0].Memory.Content != "Lives in Denver now" {
		t.Fatalf("expected only the current location, got %+v", res)
	}
	all, _, _ := e.Recall(context.Background(), "where does the user live", 5, true)
	if len(all) != 2 || all[0].Memory.Content != "Lives in Denver now" {
		t.Fatalf("include-all should rank current fact first, got %+v", all)
	}
}

// fakeEmbedder maps texts to hand-made vectors so semantic matching is testable
// without Ollama.
type fakeEmbedder struct{ fail bool }

func (fakeEmbedder) Model() string { return "fake" }
func (f fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.fail {
		return nil, errors.New("ollama down")
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		switch {
		case contains(t, "Denver"), contains(t, "Austin"), contains(t, "home town"):
			out[i] = []float32{1, 0, 0}
		case contains(t, "dogs"):
			out[i] = []float32{0, 1, 0}
		default:
			out[i] = []float32{0, 0, 1}
		}
	}
	return out, nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSemanticRecallWithoutSharedWords(t *testing.T) {
	e := New(seed(t), fakeEmbedder{}, t.TempDir()+"/emb.json")
	res, mode, err := e.Recall(context.Background(), "home town", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeHybrid || len(res) == 0 || res[0].Memory.Content != "Lives in Denver now" {
		t.Fatalf("mode=%s res=%+v", mode, res)
	}
}

func TestFallsBackWhenEmbedderFails(t *testing.T) {
	e := New(seed(t), fakeEmbedder{fail: true}, "")
	res, mode, err := e.Recall(context.Background(), "dark mode", 5, false)
	if err != nil || mode != ModeKeyword || len(res) == 0 {
		t.Fatalf("mode=%s res=%+v err=%v", mode, res, err)
	}
	if e.LastEmbedError() == nil {
		t.Fatal("expected embed error to be recorded")
	}
}

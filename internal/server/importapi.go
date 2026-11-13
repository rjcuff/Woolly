package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/importer"
	"github.com/rjcuff/woolly/internal/learn"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/ollama"
	"regexp"
	"time"
)

// maxImport matches the command line's limit for exports.
const maxImport = 2 << 30

var importNames = map[string]string{"chatgpt": "ChatGPT import", "claude": "Claude import"}

type importItem struct {
	Content string   `json:"content"`
	Key     string   `json:"key,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Known   bool     `json:"known,omitempty"`
}

// The local model reads at most this many recent messages, a dozen at a time.
const (
	smartMessages = 400
	smartBatch    = 4
)

// aboutMe keeps messages that sound like the user talking about themselves.
var aboutMe = regexp.MustCompile(`(?i)\b(i|i'm|im|i've|i'd|my|me|we|our)\b`)

// smartFacts reads the history with the local model. It returns nil if no
// model is available, so the caller falls back to the simple rules.
func smartFacts(ctx context.Context, ai *ollama.Client, msgs []importer.Message) []string {
	if ai == nil {
		return nil
	}
	check, cancel := context.WithTimeout(ctx, 2*time.Second)
	ok, err := ai.HasModel(check, ai.ChatModel)
	cancel()
	if err != nil || !ok {
		return nil
	}
	var picked []string
	for i := len(msgs) - 1; i >= 0 && len(picked) < smartMessages; i-- {
		t := strings.TrimSpace(msgs[i].Text)
		if len(t) > 8 && len(t) < 800 && aboutMe.MatchString(t) {
			picked = append(picked, t)
		}
	}
	ctx, cancel = context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	seen := map[string]bool{}
	var out []string
	for start := 0; start < len(picked); start += smartBatch {
		end := min(start+smartBatch, len(picked))
		var b strings.Builder
		for _, t := range picked[start:end] {
			b.WriteString("user: " + t + "\n\n")
		}
		facts, err := learn.ExtractFacts(ctx, ai, b.String())
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			continue
		}
		for _, f := range facts {
			if k := normText(f); !seen[k] {
				seen[k] = true
				out = append(out, f)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func registerImport(mux *http.ServeMux, store *memory.Store, log *activity.Log, ai *ollama.Client) {
	// The body is the export file itself, a .zip or conversations.json.
	mux.HandleFunc("POST /api/import/preview", func(w http.ResponseWriter, r *http.Request) {
		tmp, err := os.CreateTemp("", "woolly-import-*")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer os.Remove(tmp.Name())
		n, err := io.Copy(tmp, io.LimitReader(r.Body, maxImport+1))
		tmp.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, "couldn't read the upload: "+err.Error())
			return
		}
		if n > maxImport {
			writeError(w, http.StatusRequestEntityTooLarge, "that file is larger than 2 GB")
			return
		}
		provider, msgs, err := importer.Detect(tmp.Name())
		if err != nil {
			writeError(w, http.StatusBadRequest, "couldn't read that file. Use the .zip from ChatGPT or Claude's data export, or the conversations.json inside it")
			return
		}
		mems, _ := store.List(false)
		known := map[string]bool{}
		for _, m := range mems {
			known[normText(m.Content)] = true
		}
		items := []importItem{}
		smart := false
		facts := smartFacts(r.Context(), ai, msgs)
		smart = facts != nil
		for _, f := range facts {
			items = append(items, importItem{Content: f, Known: known[normText(f)]})
		}
		// The simple rules catch some things a small model skips. Keep the
		// ones the model didn't already find in other words.
		for _, c := range importer.Extract(msgs) {
			dup := false
			for _, f := range facts {
				if similar(f, c.Content) {
					dup = true
					break
				}
			}
			if !dup {
				items = append(items, importItem{Content: c.Content, Key: c.Key, Tags: c.Tags, Known: known[normText(c.Content)]})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"provider": provider, "name": importNames[provider], "messages": len(msgs), "items": items, "smart": smart})
	})

	mux.HandleFunc("POST /api/import/save", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Provider string       `json:"provider"`
			Items    []importItem `json:"items"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		via := importNames[req.Provider]
		if via == "" {
			writeError(w, http.StatusBadRequest, "provider must be chatgpt or claude")
			return
		}
		var added, known, conflicts, blocked int
		var ids []string
		for _, it := range req.Items {
			res, err := store.Remember(memory.RememberInput{
				Content: it.Content, Key: it.Key, Tags: append([]string{req.Provider}, it.Tags...), Source: memory.SourceImport, Via: via,
			})
			var sensitive *memory.SensitiveError
			var injected *memory.InjectionError
			switch {
			case errors.As(err, &sensitive) || errors.As(err, &injected):
				blocked++
			case err != nil:
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			case res.Duplicate:
				known++
			case res.ConflictsWith != nil:
				conflicts++
			default:
				added++
				ids = append(ids, res.Memory.ID)
			}
		}
		if added > 0 {
			log.Add(activity.Entry{App: via, Action: "import", IDs: ids, Count: added})
		}
		writeJSON(w, http.StatusOK, map[string]int{"added": added, "known": known, "conflicts": conflicts, "blocked": blocked})
	})
}

var filler = map[string]bool{"i": true, "im": true, "i'm": true, "am": true, "my": true, "is": true, "a": true, "an": true, "the": true, "to": true, "and": true, "with": true, "in": true, "of": true}

// similar reports whether two facts say the same thing, like "I live in
// Denver" and "Lives in Denver".
func similar(a, b string) bool {
	words := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '\'') }) {
			w = strings.TrimSuffix(strings.TrimSuffix(w, "s"), "'")
			if !filler[w] {
				out[w] = true
			}
		}
		return out
	}
	wa, wb := words(a), words(b)
	if len(wa) == 0 || len(wb) == 0 {
		return false
	}
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	return float64(shared)/float64(min(len(wa), len(wb))) >= 0.6
}

func normText(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

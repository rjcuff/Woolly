package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/importer"
	"github.com/rjcuff/woolly/internal/memory"
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

func registerImport(mux *http.ServeMux, store *memory.Store, log *activity.Log) {
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
		for _, c := range importer.Extract(msgs) {
			items = append(items, importItem{Content: c.Content, Key: c.Key, Tags: c.Tags, Known: known[normText(c.Content)]})
		}
		writeJSON(w, http.StatusOK, map[string]any{"provider": provider, "name": importNames[provider], "messages": len(msgs), "items": items})
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

func normText(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

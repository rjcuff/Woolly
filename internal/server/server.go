// Package server exposes Woolly over a small local HTTP API, the way Ollama
// exposes models: any app on this machine can remember and recall.
package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/profile"
	"github.com/rjcuff/woolly/internal/recall"
)

// DefaultAddr sits next to Ollama's 11434.
const DefaultAddr = "127.0.0.1:11435"

// Handler returns the HTTP handler for the Woolly API.
func Handler(store *memory.Store, engine *recall.Engine, version string) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", uiHandler())
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"version": version})
	})

	mux.HandleFunc("POST /api/remember", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Content string   `json:"content"`
			Key     string   `json:"key"`
			Tags    []string `json:"tags"`
			Source  string   `json:"source"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		in := memory.RememberInput{Content: req.Content, Key: req.Key, Tags: req.Tags}
		if req.Source != "" {
			src, ok := memory.ParseSource(req.Source)
			if !ok {
				writeError(w, http.StatusBadRequest, "source must be user, import or agent")
				return
			}
			in.Source = src
		}
		res, err := store.Remember(in)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /api/recall", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query      string `json:"query"`
			Limit      int    `json:"limit"`
			IncludeAll bool   `json:"include_all"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		results, mode, err := engine.Recall(r.Context(), req.Query, req.Limit, req.IncludeAll)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if results == nil {
			results = []recall.Result{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"mode": mode, "results": results})
	})

	mux.HandleFunc("GET /api/memories", func(w http.ResponseWriter, r *http.Request) {
		all := r.URL.Query().Get("all")
		mems, err := store.List(all == "1" || all == "true")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"memories": orEmpty(mems)})
	})

	mux.HandleFunc("GET /api/memories/{id}", func(w http.ResponseWriter, r *http.Request) {
		m, err := store.Get(r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("PATCH /api/memories/{id}", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Content *string   `json:"content"`
			Key     *string   `json:"key"`
			Tags    *[]string `json:"tags"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		m, err := store.Update(r.PathValue("id"), memory.UpdateInput{Content: req.Content, Key: req.Key, Tags: req.Tags})
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		mems, err := store.List(false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		p := profile.Build(mems)
		if r.URL.Query().Get("format") == "markdown" {
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = w.Write([]byte(p.Markdown()))
			return
		}
		writeJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("DELETE /api/memories/{id}", func(w http.ResponseWriter, r *http.Request) {
		m, err := store.Forget(r.PathValue("id"))
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"forgotten": m})
	})

	mux.HandleFunc("GET /api/conflicts", func(w http.ResponseWriter, r *http.Request) {
		mems, err := store.Conflicts()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"conflicts": orEmpty(mems)})
	})

	mux.HandleFunc("POST /api/resolve", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			KeepID string `json:"keep_id"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		kept, replaced, err := store.Resolve(req.KeepID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"kept": kept, "superseded": orEmpty(replaced)})
	})

	return localOnly(mux)
}

// Serve listens on addr until the server fails.
func Serve(addr string, h http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return ServeListener(ln, h)
}

// ServeListener serves on an existing listener until the server fails.
func ServeListener(ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
	}
	return srv.Serve(ln)
}

// localOnly blocks requests that a web page could make on the user's behalf:
// a non-local Host header (DNS rebinding) or a cross-site browser Origin.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			writeError(w, http.StatusForbidden, "woolly only accepts requests addressed to localhost")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !isLocalHost(u.Host) {
				writeError(w, http.StatusForbidden, "cross-origin requests are not allowed")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func orEmpty(m []memory.Memory) []memory.Memory {
	if m == nil {
		return []memory.Memory{}
	}
	return m
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, memory.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

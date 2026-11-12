// Package server exposes Woolly over a small local HTTP API, the way Ollama
// exposes models: any app on this machine can remember and recall.
package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/chats"
	"github.com/rjcuff/woolly/internal/learn"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/ollama"
	"github.com/rjcuff/woolly/internal/profile"
	"github.com/rjcuff/woolly/internal/recall"
)

// DefaultAddr sits next to Ollama's 11434.
const DefaultAddr = "127.0.0.1:11435"

// ReviewPage is the app name for changes made in the review page.
const ReviewPage = "review page"

// appOf names the app behind a request. Apps can name themselves with the
// X-Woolly-Client header. The review page signs in with a cookie, and
// anything else is "API".
func appOf(r *http.Request) string {
	if h := activity.AppName(r.Header.Get("X-Woolly-Client"), ""); h != "" {
		return h
	}
	if r.Header.Get("Authorization") == "" {
		if _, err := r.Cookie(tokenCookie); err == nil {
			return ReviewPage
		}
	}
	return "API"
}

// Options are the optional parts of the API. Missing ones turn their
// endpoints off.
type Options struct {
	Activity *activity.Log
	Learner  *learn.Learner
	Ollama   *ollama.Client
	Chats    *chats.Store
}

// Handler returns the HTTP handler for the Woolly API. If log is set, saves,
// reads and deletes are recorded in it. Reads by the review page are not,
// since it reads everything each time it opens.
func Handler(store *memory.Store, engine *recall.Engine, version, token string, opts Options) http.Handler {
	log, learner := opts.Activity, opts.Learner
	mux := http.NewServeMux()
	record := func(r *http.Request, action string, ids []string, query string, count int) {
		app := appOf(r)
		switch action {
		case "recall", "profile", "list":
			if app == ReviewPage {
				return
			}
		}
		log.Add(activity.Entry{App: app, Action: action, IDs: ids, Query: query, Count: count})
	}
	mux.HandleFunc("GET /auth", authHandler(token))

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
			Project string   `json:"project"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		in := memory.RememberInput{Content: req.Content, Key: req.Key, Tags: req.Tags, Via: appOf(r), Project: req.Project}
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
		record(r, "remember", []string{res.Memory.ID}, "", 0)
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /api/recall", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Query      string `json:"query"`
			Limit      int    `json:"limit"`
			IncludeAll bool   `json:"include_all"`
			Project    string `json:"project"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		results, mode, err := engine.RecallIn(r.Context(), req.Query, req.Limit, req.IncludeAll, req.Project)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if results == nil {
			results = []recall.Result{}
		}
		ids := make([]string, len(results))
		for i, res := range results {
			ids[i] = res.Memory.ID
		}
		record(r, "recall", ids, req.Query, len(results))
		writeJSON(w, http.StatusOK, map[string]any{"mode": mode, "results": results})
	})

	mux.HandleFunc("GET /api/memories", func(w http.ResponseWriter, r *http.Request) {
		all := r.URL.Query().Get("all")
		mems, err := store.List(all == "1" || all == "true")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if p := memory.NormalizeProject(r.URL.Query().Get("project")); p != "" {
			kept := mems[:0]
			for _, m := range mems {
				if m.Project == p {
					kept = append(kept, m)
				}
			}
			mems = kept
		}
		record(r, "list", nil, "", len(mems))
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
			Project *string   `json:"project"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		m, err := store.Update(r.PathValue("id"), memory.UpdateInput{Content: req.Content, Key: req.Key, Tags: req.Tags, Project: req.Project})
		if err != nil {
			writeStoreError(w, err)
			return
		}
		record(r, "update", []string{m.ID}, "", 0)
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("GET /api/profile", func(w http.ResponseWriter, r *http.Request) {
		mems, err := store.List(false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		p := profile.Build(mems)
		record(r, "profile", nil, "", len(mems))
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
		record(r, "forget", []string{m.ID}, "", 0)
		writeJSON(w, http.StatusOK, map[string]any{"forgotten": m})
	})

	mux.HandleFunc("GET /api/trash", func(w http.ResponseWriter, r *http.Request) {
		mems, err := store.Trash()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"trash": orEmpty(mems)})
	})

	mux.HandleFunc("POST /api/restore", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		m, err := store.Restore(req.ID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		record(r, "restore", []string{m.ID}, "", 0)
		writeJSON(w, http.StatusOK, m)
	})

	mux.HandleFunc("DELETE /api/trash", func(w http.ResponseWriter, r *http.Request) {
		n, err := store.EmptyTrash()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		record(r, "empty_trash", nil, "", n)
		writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
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
		record(r, "resolve", []string{kept.ID}, "", len(replaced))
		writeJSON(w, http.StatusOK, map[string]any{"kept": kept, "superseded": orEmpty(replaced)})
	})

	mux.HandleFunc("POST /api/learn", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Text    string `json:"text"`
			Project string `json:"project"`
			DryRun  bool   `json:"dry_run"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if learner == nil {
			writeError(w, http.StatusServiceUnavailable, "learning isn't available")
			return
		}
		steps, err := learner.Learn(r.Context(), req.Text, learn.Options{Project: req.Project, Via: appOf(r), DryRun: req.DryRun})
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, err.Error()+". Learning needs Ollama with a chat model running")
			return
		}
		var ids []string
		for _, s := range steps {
			if s.Memory != nil && s.Action != "skip" && !req.DryRun {
				ids = append(ids, s.Memory.ID)
			}
		}
		if len(ids) > 0 {
			record(r, "learn", ids, "", len(ids))
		}
		if steps == nil {
			steps = []learn.Step{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps})
	})

	mux.HandleFunc("GET /api/activity", func(w http.ResponseWriter, r *http.Request) {
		n := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if k, err := strconv.Atoi(v); err == nil && k > 0 && k <= 5000 {
				n = k
			}
		}
		entries, err := log.Recent(n, r.URL.Query().Get("app"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if entries == nil {
			entries = []activity.Entry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"activity": entries})
	})

	registerChat(mux, store, engine, opts, record)

	return LocalOnly(requireToken(token, mux))
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

// LocalOnly blocks requests that a web page could make on the user's behalf:
// a non-local Host header (DNS rebinding) or a cross-site browser Origin.
func LocalOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			writeError(w, http.StatusForbidden, "woolly only accepts requests addressed to localhost")
			return
		}
		// Browsers mark requests that come from other sites.
		if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" || site == "same-site" {
			writeError(w, http.StatusForbidden, "cross-site requests are not allowed")
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

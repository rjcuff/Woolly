package server

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/connect"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/peer"
)

// registerManage serves the app's Apps and Settings screens.
func registerManage(mux *http.ServeMux, store *memory.Store, version string, opts Options) {
	if opts.Home == "" {
		return
	}

	mux.HandleFunc("GET /api/apps", func(w http.ResponseWriter, r *http.Request) {
		env, err := connect.EnvFromOS(runtime.GOOS)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		type app struct {
			connect.Status
			LastUsed *time.Time `json:"last_used,omitempty"`
		}
		used := map[string]time.Time{}
		entries, _ := opts.Activity.Recent(5000, "")
		for _, e := range entries {
			if _, ok := used[e.App]; !ok {
				used[e.App] = e.Time
			}
		}
		var out []app
		for _, s := range connect.Statuses(env) {
			a := app{Status: s}
			if t, ok := used[activity.AppName(s.ID, s.Name)]; ok {
				a.LastUsed = &t
			} else if t, ok := used[s.Name]; ok {
				a.LastUsed = &t
			}
			out = append(out, a)
		}
		writeJSON(w, http.StatusOK, map[string]any{"apps": out})
	})

	mux.HandleFunc("POST /api/apps/{id}/{action}", func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		if action != "connect" && action != "disconnect" {
			writeError(w, http.StatusNotFound, "use connect or disconnect")
			return
		}
		env, err := connect.EnvFromOS(runtime.GOOS)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		command, err := connect.WoollyCommand()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		res, err := connect.ConnectByID(env, r.PathValue("id"), command, action == "disconnect")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"result": res.String()})
	})

	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{"version": version, "home": opts.Home}
		if ai := opts.Ollama; ai != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			o := map[string]any{"host": ai.Host, "chat_model": ai.ChatModel, "embed_model": ai.EmbedModel}
			if v, err := ai.Version(ctx); err == nil {
				o["running"], o["version"] = true, v
				o["has_chat_model"], _ = ai.HasModel(ctx, ai.ChatModel)
				o["has_embed_model"], _ = ai.HasModel(ctx, ai.EmbedModel)
			} else {
				o["running"] = false
			}
			out["ollama"] = o
		}
		cfg, err := peer.Load(opts.Home)
		switch {
		case err != nil:
			out["sync"] = map[string]any{"enabled": false, "error": err.Error()}
		case cfg == nil:
			out["sync"] = map[string]any{"enabled": false}
		default:
			out["sync"] = map[string]any{"enabled": true, "name": cfg.Name, "peers": cfg.Peers, "addresses": peer.LocalAddrs(), "port": peer.DefaultPort}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /api/sync/init", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := peer.Load(opts.Home)
		if err == nil && cfg == nil {
			if cfg, err = peer.NewConfig(); err == nil {
				err = cfg.Save(opts.Home)
			}
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if opts.StartSync != nil {
			_ = opts.StartSync()
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": cfg.JoinCode(), "addresses": peer.LocalAddrs()})
	})

	mux.HandleFunc("POST /api/sync/join", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Addr string `json:"addr"`
			Code string `json:"code"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		if !peer.ValidCode(req.Code) || strings.TrimSpace(req.Addr) == "" {
			writeError(w, http.StatusBadRequest, "enter the address and code from your other device")
			return
		}
		cfg, err := peer.Load(opts.Home)
		if err == nil && cfg == nil {
			cfg, err = peer.NewConfig()
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if cfg.Key != req.Code && len(cfg.Peers) > 0 {
			writeError(w, http.StatusConflict, "this device already syncs with a different group. Stop syncing first")
			return
		}
		cfg.Key = req.Code
		cfg.AddPeer(peer.NormalizeAddr(req.Addr))
		if err := cfg.Save(opts.Home); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if opts.StartSync != nil {
			_ = opts.StartSync()
		}
		writeJSON(w, http.StatusOK, syncAll(r.Context(), store, cfg, opts))
	})

	mux.HandleFunc("POST /api/sync/now", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := peer.Load(opts.Home)
		if err != nil || cfg == nil {
			writeError(w, http.StatusBadRequest, "sync isn't set up")
			return
		}
		writeJSON(w, http.StatusOK, syncAll(r.Context(), store, cfg, opts))
	})

	mux.HandleFunc("POST /api/sync/leave", func(w http.ResponseWriter, r *http.Request) {
		if err := peer.Remove(opts.Home); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"left": true})
	})
}

type syncResult struct {
	Addr     string `json:"addr"`
	Name     string `json:"name,omitempty"`
	Received int    `json:"received"`
	Sent     int    `json:"sent"`
	Error    string `json:"error,omitempty"`
}

func syncAll(ctx context.Context, store *memory.Store, cfg *peer.Config, opts Options) map[string]any {
	client := &peer.Client{}
	var out []syncResult
	for i := range cfg.Peers {
		p := &cfg.Peers[i]
		c, cancel := context.WithTimeout(ctx, time.Minute)
		res, err := client.Sync(c, store, cfg, p.Addr)
		cancel()
		r := syncResult{Addr: p.Addr, Name: res.Name, Received: res.Received, Sent: res.Sent}
		if err != nil {
			r.Error, p.LastErr = err.Error(), err.Error()
		} else {
			p.LastErr, p.LastSync = "", time.Now().UTC()
			if res.Name != "" {
				p.Name = res.Name
			}
			if n := res.Received + res.Sent + res.Deleted; n > 0 {
				opts.Activity.Add(activity.Entry{App: p.Addr, Action: "sync", Count: n})
			}
		}
		out = append(out, r)
	}
	_ = cfg.Save(opts.Home)
	return map[string]any{"results": out}
}

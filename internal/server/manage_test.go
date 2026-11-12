package server

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/recall"
)

func TestSettingsAndSyncSetup(t *testing.T) {
	home := t.TempDir()
	store, _ := memory.Open(filepath.Join(home, "memories"))
	token, _ := EnsureToken(home)
	started := 0
	h := Handler(store, recall.New(store, nil, ""), "9.9.9", token, Options{
		Activity: activity.Open(home), Home: home, StartSync: func() error { started++; return nil },
	})
	auth := map[string]string{"Authorization": "Bearer " + token}

	rec := do(h, "GET", "/api/settings", "", auth)
	if !strings.Contains(rec.Body.String(), `"version":"9.9.9"`) || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("settings: %s", rec.Body)
	}
	rec = do(h, "POST", "/api/sync/init", "{}", auth)
	var init struct {
		Code string `json:"code"`
	}
	json.Unmarshal(rec.Body.Bytes(), &init)
	if len(init.Code) != 43 || started != 1 {
		t.Fatalf("init: %s started=%d", rec.Body, started)
	}
	if !strings.Contains(do(h, "GET", "/api/settings", "", auth).Body.String(), `"enabled":true`) {
		t.Fatal("sync should be on")
	}
	if rec := do(h, "POST", "/api/sync/join", `{"addr":"1.2.3.4","code":"nope"}`, auth); rec.Code != 400 {
		t.Fatalf("bad code: %d", rec.Code)
	}
	do(h, "POST", "/api/sync/leave", "{}", auth)
	if !strings.Contains(do(h, "GET", "/api/settings", "", auth).Body.String(), `"enabled":false`) {
		t.Fatal("sync should be off")
	}
	if rec := do(h, "POST", "/api/apps/nope/connect", "", auth); rec.Code != 400 {
		t.Fatalf("unknown app: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/apps", "", nil); rec.Code != 401 {
		t.Fatalf("apps need the token: %d", rec.Code)
	}
}

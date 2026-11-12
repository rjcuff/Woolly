package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/recall"
)

func setup(t *testing.T) (http.Handler, string) {
	t.Helper()
	home := t.TempDir()
	store, err := memory.Open(filepath.Join(home, "memories"))
	if err != nil {
		t.Fatal(err)
	}
	token, err := EnsureToken(home)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(filepath.Join(home, "token"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode %v, want 0600", info.Mode().Perm())
	}
	return Handler(store, recall.New(store, nil, ""), "test", token, Options{Activity: activity.Open(home)}), token
}

func do(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:11435"
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPIRequiresToken(t *testing.T) {
	h, token := setup(t)
	if code := do(h, "GET", "/api/memories", "", nil).Code; code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code := do(h, "GET", "/api/memories", "", map[string]string{"Authorization": "Bearer wrong"}).Code; code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", code)
	}
	if code := do(h, "GET", "/api/memories", "", map[string]string{"Authorization": "Bearer " + token}).Code; code != http.StatusOK {
		t.Fatalf("right token: %d", code)
	}
	if code := do(h, "GET", "/api/version", "", nil).Code; code != http.StatusOK {
		t.Fatalf("version should stay open: %d", code)
	}
	if code := do(h, "GET", "/", "", nil).Code; code != http.StatusOK {
		t.Fatalf("page itself should load: %d", code)
	}
}

func TestAuthLinkSetsCookie(t *testing.T) {
	h, token := setup(t)
	rec := do(h, "GET", "/auth?token="+token, "", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("auth: %d", rec.Code)
	}
	c := rec.Result().Cookies()
	if len(c) != 1 || !c[0].HttpOnly || c[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie: %+v", c)
	}
	ok := do(h, "GET", "/api/memories", "", map[string]string{"Cookie": tokenCookie + "=" + token})
	if ok.Code != http.StatusOK {
		t.Fatalf("cookie auth: %d", ok.Code)
	}
	if code := do(h, "GET", "/auth?token=nope", "", nil).Code; code != http.StatusForbidden {
		t.Fatalf("bad link: %d", code)
	}
}

func TestBlocksOtherSites(t *testing.T) {
	h, token := setup(t)
	auth := "Bearer " + token
	cases := []map[string]string{
		{"Authorization": auth, "Sec-Fetch-Site": "cross-site"},
		{"Authorization": auth, "Origin": "https://evil.example"},
	}
	for _, hdr := range cases {
		if code := do(h, "GET", "/api/memories", "", hdr).Code; code != http.StatusForbidden {
			t.Fatalf("%v: %d", hdr, code)
		}
	}
	req := httptest.NewRequest("GET", "/api/memories", nil)
	req.Host = "evil.example"
	req.Header.Set("Authorization", auth)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-local host: %d", rec.Code)
	}
}

func TestActivityRecordsApps(t *testing.T) {
	h, token := setup(t)
	bearer := map[string]string{"Authorization": "Bearer " + token, "X-Woolly-Client": "my-script"}
	if rec := do(h, "POST", "/api/remember", `{"content":"Likes tea"}`, bearer); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"via":"my-script"`) {
		t.Fatalf("remember: %d %s", rec.Code, rec.Body)
	}
	do(h, "POST", "/api/recall", `{"query":"tea"}`, map[string]string{"Authorization": "Bearer " + token})
	// The review page reads everything when it opens; that isn't logged.
	do(h, "GET", "/api/memories", "", map[string]string{"Cookie": "woolly_token=" + token})

	rec := do(h, "GET", "/api/activity", "", map[string]string{"Authorization": "Bearer " + token})
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"app":"my-script","action":"remember"`) || !strings.Contains(body, `"app":"API","action":"recall"`) {
		t.Fatalf("activity: %d %s", rec.Code, body)
	}
	if strings.Contains(body, "review page") {
		t.Fatalf("review page reads should not be logged: %s", body)
	}
	if code := do(h, "GET", "/api/activity", "", nil).Code; code != http.StatusUnauthorized {
		t.Fatalf("activity needs the token: %d", code)
	}
}

func TestProjectsOverTheAPI(t *testing.T) {
	h, token := setup(t)
	auth := map[string]string{"Authorization": "Bearer " + token}
	do(h, "POST", "/api/remember", `{"content":"Acme deploys with Fly","project":"Acme"}`, auth)
	do(h, "POST", "/api/remember", `{"content":"Notes deploys with Vercel","project":"notes"}`, auth)
	rec := do(h, "POST", "/api/recall", `{"query":"deploys","project":"acme"}`, auth)
	if !strings.Contains(rec.Body.String(), "Fly") || strings.Contains(rec.Body.String(), "Vercel") {
		t.Fatalf("recall: %s", rec.Body)
	}
	rec = do(h, "GET", "/api/memories?project=notes", "", auth)
	if strings.Contains(rec.Body.String(), "Fly") || !strings.Contains(rec.Body.String(), `"project":"notes"`) {
		t.Fatalf("list: %s", rec.Body)
	}
}

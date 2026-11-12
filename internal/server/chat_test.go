package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/chats"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/ollama"
	"github.com/rjcuff/woolly/internal/recall"
)

// fakeOllama streams a fixed reply and records the system prompt it got.
func fakeOllama(t *testing.T, system *string) *ollama.Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"models":[{"name":"qwen2.5:3b","size":1,"details":{"family":"qwen2"}},{"name":"nomic-embed-text:latest","details":{"family":"nomic-bert"}}]}`)
	})
	mux.HandleFunc("/api/chat", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []ollama.Message `json:"messages"`
			Stream   bool             `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			fmt.Fprint(w, `{"message":{"content":"{\"facts\":[]}"}}`)
			return
		}
		*system = req.Messages[0].Content
		for _, part := range []string{"Denver ", "has great ", "hikes."} {
			fmt.Fprintf(w, `{"message":{"content":%q},"done":false}`+"\n", part)
		}
		fmt.Fprint(w, `{"done":true}`+"\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &ollama.Client{Host: srv.URL, ChatModel: "qwen2.5:3b", HTTP: srv.Client()}
}

func TestChatStreamsWithMemories(t *testing.T) {
	home := t.TempDir()
	store, _ := memory.Open(filepath.Join(home, "memories"))
	store.Remember(memory.RememberInput{Content: "Lives in Denver and loves hiking"})
	token, _ := EnsureToken(home)
	var system string
	ai := fakeOllama(t, &system)
	h := Handler(store, recall.New(store, nil, ""), "test", token, Options{Activity: activity.Open(home), Ollama: ai, Chats: chats.Open(home)})
	auth := map[string]string{"Authorization": "Bearer " + token}

	rec := do(h, "GET", "/api/models", "", auth)
	if !strings.Contains(rec.Body.String(), "qwen2.5:3b") || strings.Contains(rec.Body.String(), "nomic") {
		t.Fatalf("models: %s", rec.Body)
	}

	rec = do(h, "POST", "/api/chat", `{"model":"qwen2.5:3b","message":"Where should I go hiking?","learn":false}`, auth)
	var events []map[string]any
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		var e map[string]any
		json.Unmarshal(sc.Bytes(), &e)
		events = append(events, e)
	}
	if len(events) < 3 || events[0]["type"] != "start" || events[len(events)-1]["type"] != "done" {
		t.Fatalf("events: %v", events)
	}
	if !strings.Contains(fmt.Sprint(events[0]["memories"]), "Denver") || !strings.Contains(system, "Lives in Denver") {
		t.Fatalf("memories not used: %v / %s", events[0], system)
	}
	id := events[0]["chat_id"].(string)
	c, err := chats.Open(home).Get(id)
	if err != nil || len(c.Messages) != 2 || c.Messages[1].Content != "Denver has great hikes." {
		t.Fatalf("saved chat: %+v %v", c, err)
	}
	if !strings.Contains(do(h, "GET", "/api/chats", "", auth).Body.String(), "Where should I go hiking?") {
		t.Fatal("chat list")
	}
}

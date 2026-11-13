package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const chatgptExport = `[{"title":"t","mapping":{
 "a":{"message":{"author":{"role":"user"},"content":{"content_type":"text","parts":["I live in Denver. Can you help me plan a trip?"]},"create_time":1700000000}},
 "b":{"message":{"author":{"role":"assistant"},"content":{"content_type":"text","parts":["Sure!"]},"create_time":1700000001}},
 "c":{"message":{"author":{"role":"user"},"content":{"content_type":"text","parts":["I prefer short answers"]},"create_time":1700000002}}
}}]`

func TestImportPreviewAndSave(t *testing.T) {
	h, token := setup(t)
	auth := map[string]string{"Authorization": "Bearer " + token}
	do(h, "POST", "/api/remember", `{"content":"I prefer short answers"}`, auth)

	rec := do(h, "POST", "/api/import/preview", chatgptExport, auth)
	var prev struct {
		Provider string `json:"provider"`
		Messages int    `json:"messages"`
		Items    []struct {
			Content string `json:"content"`
			Known   bool   `json:"known"`
		} `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &prev)
	if rec.Code != 200 || prev.Provider != "chatgpt" || prev.Messages != 2 || len(prev.Items) == 0 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body)
	}
	knownSeen := false
	for _, it := range prev.Items {
		if strings.Contains(strings.ToLower(it.Content), "short answers") {
			knownSeen = it.Known
		}
	}
	if !knownSeen {
		t.Fatalf("an existing memory should be marked known: %s", rec.Body)
	}

	rec = do(h, "POST", "/api/import/save", `{"provider":"chatgpt","items":[{"content":"Lives in Denver"},{"content":"My key is sk-proj-a8F3kQ9zLm2xR7tV4bN6cW1yE5"}]}`, auth)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"added":1`) || !strings.Contains(rec.Body.String(), `"blocked":1`) {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(do(h, "GET", "/api/memories", "", auth).Body.String(), `"via":"ChatGPT import"`) {
		t.Fatal("saved memories should say where they came from")
	}
	if rec := do(h, "POST", "/api/import/preview", "not an export", auth); rec.Code != 400 {
		t.Fatalf("junk: %d", rec.Code)
	}
}

func TestImportPreviewFromZip(t *testing.T) {
	h, token := setup(t)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("export/conversations.json")
	f.Write([]byte(chatgptExport))
	zw.Close()
	rec := do(h, "POST", "/api/import/preview", buf.String(), map[string]string{"Authorization": "Bearer " + token})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"provider":"chatgpt"`) {
		t.Fatalf("zip preview: %d %s", rec.Code, rec.Body)
	}
}

func TestSimilarFacts(t *testing.T) {
	if !similar("I live in Denver", "Lives in Denver") || !similar("I'm allergic to peanuts", "Allergic to peanuts") || similar("I live in Denver", "Has a dog named Biscuit") {
		t.Fatal("similar")
	}
}

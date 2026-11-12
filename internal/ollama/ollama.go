// Package ollama is a minimal client for the parts of the Ollama API that
// Woolly uses. Ollama is optional: without it Woolly falls back to keyword
// search.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultEmbedModel is a small, fast embedding model (~270 MB).
const DefaultEmbedModel = "nomic-embed-text"

// DefaultChatModel picks out facts for `woolly learn`. Of the small models
// tested, it missed the fewest facts and never made one up.
const DefaultChatModel = "qwen2.5:3b"

// Client talks to a local Ollama server.
type Client struct {
	Host       string
	EmbedModel string
	ChatModel  string
	HTTP       *http.Client
}

// FromEnv builds a client from OLLAMA_HOST and WOOLLY_EMBED_MODEL.
func FromEnv() *Client {
	model := strings.TrimSpace(os.Getenv("WOOLLY_EMBED_MODEL"))
	if model == "" {
		model = DefaultEmbedModel
	}
	chat := strings.TrimSpace(os.Getenv("WOOLLY_LLM_MODEL"))
	if chat == "" {
		chat = DefaultChatModel
	}
	return &Client{
		Host:       normalizeHost(os.Getenv("OLLAMA_HOST")),
		EmbedModel: model,
		ChatModel:  chat,
		HTTP:       &http.Client{Timeout: 2 * time.Minute},
	}
}

func normalizeHost(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "http://127.0.0.1:11434"
	}
	if !strings.Contains(h, "://") {
		h = "http://" + h
	}
	u, err := url.Parse(h)
	if err != nil {
		return "http://127.0.0.1:11434"
	}
	host, port := u.Hostname(), u.Port()
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "11434"
	}
	u.Host = net.JoinHostPort(host, port)
	return strings.TrimRight(u.String(), "/")
}

// Model implements recall.Embedder.
func (c *Client) Model() string { return c.EmbedModel }

// Embed implements recall.Embedder using POST /api/embed.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	err := c.do(ctx, http.MethodPost, "/api/embed", map[string]any{"model": c.EmbedModel, "input": texts}, &out)
	if err != nil {
		return nil, err
	}
	return out.Embeddings, nil
}

// Version returns the Ollama server version, or an error if it isn't running.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/version", nil, &out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// HasModel reports whether a model has been pulled.
func (c *Client) HasModel(ctx context.Context, name string) (bool, error) {
	var out struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &out); err != nil {
		return false, err
	}
	want := name
	if !strings.Contains(want, ":") {
		want += ":latest"
	}
	for _, m := range out.Models {
		if m.Name == name || m.Name == want || m.Model == name || m.Model == want {
			return true, nil
		}
	}
	return false, nil
}

// ModelInfo is a pulled model.
type ModelInfo struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Family string `json:"family,omitempty"`
}

// ChatModels lists pulled models that can chat, leaving out embedding models.
func (c *Client) ChatModels(ctx context.Context) ([]ModelInfo, error) {
	var out struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				Family   string   `json:"family"`
				Families []string `json:"families"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &out); err != nil {
		return nil, err
	}
	models := []ModelInfo{}
	for _, m := range out.Models {
		fam := strings.ToLower(m.Details.Family + " " + strings.Join(m.Details.Families, " "))
		if strings.Contains(m.Name, "embed") || strings.Contains(fam, "bert") {
			continue
		}
		models = append(models, ModelInfo{Name: m.Name, Size: m.Size, Family: m.Details.Family})
	}
	return models, nil
}

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Stream runs a chat and calls fn with each piece of the reply as it arrives.
func (c *Client) Stream(ctx context.Context, model string, messages []Message, fn func(string) error) error {
	body, err := json.Marshal(map[string]any{"model": model, "messages": messages, "stream": true})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Replies can take minutes, so no client timeout; ctx ends it.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", c.Host, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("ollama: %s", e.Error)
		}
		return fmt.Errorf("ollama: %s", resp.Status)
	}
	dec := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done  bool   `json:"done"`
			Error string `json:"error"`
		}
		if err := dec.Decode(&chunk); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if chunk.Error != "" {
			return fmt.Errorf("ollama: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			if err := fn(chunk.Message.Content); err != nil {
				return err
			}
		}
		if chunk.Done {
			return nil
		}
	}
}

// Chat sends one system and one user message and returns the JSON reply.
// format is "json" or a JSON schema the reply must match.
func (c *Client) Chat(ctx context.Context, system, user string, format any) (string, error) {
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	err := c.do(ctx, http.MethodPost, "/api/chat", map[string]any{
		"model":  c.ChatModel,
		"stream": false,
		"format": format,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"options": map[string]any{"temperature": 0},
	}, &out)
	return out.Message.Content, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w", c.Host, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("ollama: %s", e.Error)
		}
		return fmt.Errorf("ollama: %s", resp.Status)
	}
	return json.Unmarshal(data, out)
}

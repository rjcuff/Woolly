// Package mcp serves Woolly to AI apps (Claude Code, Claude Desktop, Cursor,
// and any other MCP client) over the Model Context Protocol's stdio transport:
// newline-delimited JSON-RPC 2.0 on stdin/stdout.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rjcuff/wooly/internal/memory"
	"github.com/rjcuff/wooly/internal/recall"
)

var supportedVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `Woolly is the user's personal, local memory, shared across all of their AI tools.
At the start of a task, call "recall" with a short query about the topic to load relevant context.
When you learn a durable fact (a preference, a decision, a fact about the user or their projects), call "remember".
Use a stable "key" (e.g. "user.location", "project.acme.database") for facts that can change, so newer facts replace older ones.
Set source to "user" only when the user said it directly; use "agent" for things you inferred.`

// Server is an MCP server backed by a Woolly store.
type Server struct {
	Store   *memory.Store
	Engine  *recall.Engine
	Version string

	outMu sync.Mutex
	out   io.Writer
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve handles messages from in until EOF or ctx is done. Only protocol
// messages are written to out; diagnostics belong on stderr.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = out
	r := bufio.NewReader(in)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line, err := r.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			s.handleLine(ctx, line)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func (s *Server) handleLine(ctx context.Context, line []byte) {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		s.send(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
		return
	}
	if len(req.ID) == 0 {
		return // notification (e.g. notifications/initialized): no reply
	}
	result, rerr := s.dispatch(ctx, req)
	resp := response{JSONRPC: "2.0", ID: req.ID}
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	s.send(resp)
}

func (s *Server) send(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.outMu.Lock()
	defer s.outMu.Unlock()
	_, _ = s.out.Write(append(data, '\n'))
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		version := supportedVersions[0]
		for _, v := range supportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "woolly", "title": "Woolly", "version": s.Version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		text, err := s.callTool(ctx, p.Name, p.Arguments)
		if err != nil {
			return toolResult(err.Error(), true), nil
		}
		return toolResult(text, false), nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func toolResult(text string, isError bool) map[string]any {
	res := map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
	if isError {
		res["isError"] = true
	}
	return res
}

func obj(props map[string]any, required ...string) map[string]any {
	o := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		o["required"] = required
	}
	return o
}

var tools = []map[string]any{
	{
		"name":  "remember",
		"title": "Remember",
		"description": "Save a durable fact about the user or their work to Woolly, the user's local memory. " +
			"Use for preferences, facts, decisions and context worth keeping across sessions and apps. " +
			"Give a stable key for facts that can change; a newer memory with the same key replaces the older one.",
		"inputSchema": obj(map[string]any{
			"content": map[string]any{"type": "string", "description": "The fact, as one short self-contained sentence."},
			"key":     map[string]any{"type": "string", "description": `Optional stable slot such as "user.location" or "project.acme.database".`},
			"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional tags."},
			"source":  map[string]any{"type": "string", "enum": []string{"user", "agent"}, "description": `"user" if the user said it directly, "agent" if you inferred it. Default "agent".`},
		}, "content"),
	},
	{
		"name":        "recall",
		"title":       "Recall",
		"description": "Search the user's Woolly memories for anything relevant to a query. Returns the current (non-stale) facts, best match first.",
		"inputSchema": obj(map[string]any{
			"query": map[string]any{"type": "string", "description": "What you want to know, e.g. \"user's coding preferences\"."},
			"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Max results (default 5)."},
		}, "query"),
	},
	{
		"name":        "list_memories",
		"title":       "List memories",
		"description": "List the user's memories, newest first.",
		"inputSchema": obj(map[string]any{
			"include_superseded": map[string]any{"type": "boolean", "description": "Also show replaced and conflicting memories."},
		}),
	},
	{
		"name":        "forget",
		"title":       "Forget",
		"description": "Permanently delete a memory by id. Only do this when the user asks.",
		"inputSchema": obj(map[string]any{"id": map[string]any{"type": "string"}}, "id"),
	},
	{
		"name":        "resolve_conflict",
		"title":       "Resolve conflict",
		"description": "Make one memory the current fact for its key, replacing the others. Only do this when the user confirms which is right.",
		"inputSchema": obj(map[string]any{"keep_id": map[string]any{"type": "string"}}, "keep_id"),
	},
}

func (s *Server) callTool(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	switch name {
	case "remember":
		var a struct {
			Content string `json:"content"`
			Key     string `json:"key"`
			Tags    any    `json:"tags"`
			Source  string `json:"source"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		src := memory.SourceAgent
		if a.Source != "" {
			parsed, ok := memory.ParseSource(a.Source)
			if !ok {
				return "", errors.New(`source must be "user" or "agent"`)
			}
			src = parsed
		}
		res, err := s.Store.Remember(memory.RememberInput{Content: a.Content, Key: a.Key, Tags: toStrings(a.Tags), Source: src})
		if err != nil {
			return "", err
		}
		return DescribeRemember(res), nil

	case "recall":
		var a struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("invalid arguments: %v", err)
		}
		results, _, err := s.Engine.Recall(ctx, a.Query, a.Limit, false)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "No matching memories.", nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%d memories:\n", len(results))
		for _, r := range results {
			b.WriteString(Line(r.Memory) + "\n")
		}
		return b.String(), nil

	case "list_memories":
		var a struct {
			IncludeSuperseded bool `json:"include_superseded"`
		}
		_ = json.Unmarshal(raw, &a)
		mems, err := s.Store.List(a.IncludeSuperseded)
		if err != nil {
			return "", err
		}
		if len(mems) == 0 {
			return "No memories yet.", nil
		}
		var b strings.Builder
		for _, m := range mems {
			b.WriteString(Line(m) + "\n")
		}
		return b.String(), nil

	case "forget":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &a)
		m, err := s.Store.Forget(a.ID)
		if err != nil {
			return "", err
		}
		return "Forgot: " + m.Content, nil

	case "resolve_conflict":
		var a struct {
			KeepID string `json:"keep_id"`
		}
		_ = json.Unmarshal(raw, &a)
		kept, replaced, err := s.Store.Resolve(a.KeepID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Kept %q as the current fact; replaced %d other memories.", kept.Content, len(replaced)), nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

func toStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// Line formats a memory on one line for humans and models alike.
func Line(m memory.Memory) string {
	var meta []string
	if m.Key != "" {
		meta = append(meta, "key: "+m.Key)
	}
	meta = append(meta, "source: "+string(m.Source))
	if m.Status != memory.StatusActive {
		meta = append(meta, "status: "+string(m.Status))
	}
	if len(m.Tags) > 0 {
		meta = append(meta, "tags: "+strings.Join(m.Tags, ","))
	}
	meta = append(meta, m.UpdatedAt.Local().Format("2006-01-02"))
	return fmt.Sprintf("[%s] %s (%s)", m.ID, m.Content, strings.Join(meta, "; "))
}

// DescribeRemember explains what a remember call did.
func DescribeRemember(res *memory.RememberResult) string {
	var b strings.Builder
	switch {
	case res.Duplicate:
		fmt.Fprintf(&b, "Already remembered: %s", Line(res.Memory))
	case res.ConflictsWith != nil:
		fmt.Fprintf(&b, "Saved as a CONFLICT for review: %s\nIt disagrees with a more trusted memory, which stays current: %s",
			Line(res.Memory), Line(*res.ConflictsWith))
	default:
		fmt.Fprintf(&b, "Remembered: %s", Line(res.Memory))
	}
	for _, old := range res.Superseded {
		fmt.Fprintf(&b, "\nReplaced older memory: %s", old.Content)
	}
	return b.String()
}

// Package activity logs what each app saved, read and deleted. The log stays
// in ~/.woolly and holds memory IDs and queries, not the memories themselves.
package activity

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

// maxSize is when the log is rotated to activity.log.1.
const maxSize = 5 << 20

// Entry is one thing an app did.
type Entry struct {
	Time   time.Time `json:"time"`
	App    string    `json:"app"`
	Action string    `json:"action"` // remember, recall, profile, context, list, update, forget, restore, resolve, empty_trash, import
	IDs    []string  `json:"ids,omitempty"`
	Query  string    `json:"query,omitempty"`
	Count  int       `json:"count,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// Log appends entries to a file. A nil *Log does nothing.
type Log struct {
	path string
	mu   sync.Mutex
}

// Open returns the log in home. The file is created on first write.
func Open(home string) *Log {
	return &Log{path: filepath.Join(home, "activity.log")}
}

// Path is where the log is stored.
func (l *Log) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Add records an entry. Errors are ignored.
func (l *Log) Add(e Entry) {
	if l == nil {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if e.App == "" {
		e.App = "unknown"
	}
	if len(e.Query) > 200 {
		e.Query = e.Query[:200]
	}
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, err := os.Stat(l.path); err == nil && st.Size() > maxSize {
		_ = os.Rename(l.path, l.path+".1")
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// Recent returns up to n entries, newest first, optionally for one app.
func (l *Log) Recent(n int, app string) ([]Entry, error) {
	if l == nil {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var all []Entry
	for _, p := range []string{l.path + ".1", l.path} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1<<20)
		for sc.Scan() {
			var e Entry
			if json.Unmarshal(sc.Bytes(), &e) == nil {
				all = append(all, e)
			}
		}
		f.Close()
	}
	app = strings.ToLower(app)
	var out []Entry
	for i := len(all) - 1; i >= 0 && (n <= 0 || len(out) < n); i-- {
		if app != "" && !strings.Contains(strings.ToLower(all[i].App), app) {
			continue
		}
		out = append(out, all[i])
	}
	return out, nil
}

// known maps the names MCP clients send to the names people know.
var known = map[string]string{
	"claude-code":           "Claude Code",
	"claude-ai":             "Claude Desktop",
	"claude-desktop":        "Claude Desktop",
	"cursor-vscode":         "Cursor",
	"cursor":                "Cursor",
	"visual-studio-code":    "VS Code",
	"vscode":                "VS Code",
	"windsurf-client":       "Windsurf",
	"windsurf":              "Windsurf",
	"gemini-cli-mcp-client": "Gemini CLI",
	"gemini-cli":            "Gemini CLI",
	"codex-mcp-client":      "Codex",
	"codex":                 "Codex",
	"lm-studio":             "LM Studio",
	"opencode":              "opencode",
	"zed":                   "Zed",
}

// AppName turns an MCP client's name into one people recognize.
func AppName(name, title string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if v, ok := known[n]; ok {
		return v
	}
	s := strings.TrimSpace(title)
	if s == "" {
		s = strings.TrimSpace(name)
	}
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 40 {
		s = string(r[:40])
	}
	return s
}

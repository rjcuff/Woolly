// Package connect adds Woolly to the MCP config of AI apps on this computer.
// It only touches the "woolly" entry and backs up each file first.
package connect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// ServerName is the name Woolly is registered under in each app.
const ServerName = "woolly"

// BackupSuffix is added to a config file's name for the copy saved the first
// time Woolly changes it. Later changes keep that first copy.
const BackupSuffix = ".woolly-backup"

// Format says how an app lays out its MCP servers.
type Format int

const (
	// FormatMCPServers is {"mcpServers": {"name": {"command": ..., "args": [...]}}}.
	// Claude Desktop, Cursor and Windsurf use it.
	FormatMCPServers Format = iota
	// FormatServers is {"servers": {"name": {"type": "stdio", "command": ...}}}.
	// VS Code uses it.
	FormatServers
	// FormatOpenCode is {"mcp": {"servers": {"name": {"type": "local", "command": [...]}}}}.
	// opencode uses it.
	FormatOpenCode
)

// App is an AI app whose MCP config Woolly can edit.
type App struct {
	ID     string // short name used on the command line, e.g. "cursor"
	Name   string // display name, e.g. "Cursor"
	Config string // path to the app's MCP config file
	Format Format
	// Restart is true when the app only reads its config at startup.
	Restart bool
}

// Env holds the locations Apps needs. It is separate from the real
// environment so tests can point it at a temporary directory.
type Env struct {
	GOOS         string
	Home         string // user home directory
	AppData      string // %APPDATA% on Windows
	LocalAppData string // %LOCALAPPDATA% on Windows
}

// EnvFromOS reads Env from the running system.
func EnvFromOS(goos string) (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, fmt.Errorf("finding your home folder: %w", err)
	}
	return Env{
		GOOS:         goos,
		Home:         home,
		AppData:      os.Getenv("APPDATA"),
		LocalAppData: os.Getenv("LOCALAPPDATA"),
	}, nil
}

// configDir is where an OS keeps per-user app settings.
func (e Env) configDir() string {
	switch e.GOOS {
	case "windows":
		if e.AppData != "" {
			return e.AppData
		}
		return filepath.Join(e.Home, "AppData", "Roaming")
	case "darwin":
		return filepath.Join(e.Home, "Library", "Application Support")
	default:
		return filepath.Join(e.Home, ".config")
	}
}

// Apps lists the apps Woolly knows how to connect, with their config paths
// for this computer. It does not check whether they are installed.
func Apps(e Env) []App {
	return []App{
		{
			ID: "claude-desktop", Name: "Claude Desktop", Restart: true,
			Config: claudeDesktopConfig(e), Format: FormatMCPServers,
		},
		{
			ID: "cursor", Name: "Cursor",
			Config: filepath.Join(e.Home, ".cursor", "mcp.json"), Format: FormatMCPServers,
		},
		{
			ID: "windsurf", Name: "Windsurf", Restart: true,
			Config: filepath.Join(e.Home, ".codeium", "windsurf", "mcp_config.json"), Format: FormatMCPServers,
		},
		{
			ID: "vscode", Name: "VS Code",
			Config: filepath.Join(e.configDir(), "Code", "User", "mcp.json"), Format: FormatServers,
		},
		{
			ID: "gemini", Name: "Gemini CLI",
			Config: filepath.Join(e.Home, ".gemini", "settings.json"), Format: FormatMCPServers,
		},
		{
			ID: "lmstudio", Name: "LM Studio", Restart: true,
			Config: filepath.Join(e.Home, ".lmstudio", "mcp.json"), Format: FormatMCPServers,
		},
		{
			ID: "opencode", Name: "opencode",
			Config: openCodeConfig(e), Format: FormatOpenCode,
		},
	}
}

// openCodeConfig is opencode's global config. It uses ~/.config on every
// system, and may be named opencode.jsonc.
func openCodeConfig(e Env) string {
	dir := filepath.Join(e.Home, ".config", "opencode")
	if jsonc := filepath.Join(dir, "opencode.jsonc"); fileExists(jsonc) {
		return jsonc
	}
	return filepath.Join(dir, "opencode.json")
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// claudeDesktopConfig finds Claude Desktop's config. The Microsoft Store build
// on Windows keeps its files in a package folder instead of %APPDATA%.
func claudeDesktopConfig(e Env) string {
	std := filepath.Join(e.configDir(), "Claude", "claude_desktop_config.json")
	if e.GOOS != "windows" || e.LocalAppData == "" || dirExists(filepath.Dir(std)) {
		return std
	}
	matches, _ := filepath.Glob(filepath.Join(e.LocalAppData, "Packages", "Claude_*", "LocalCache", "Roaming", "Claude"))
	sort.Strings(matches)
	if len(matches) > 0 {
		return filepath.Join(matches[0], "claude_desktop_config.json")
	}
	return std
}

// Installed reports whether the app looks installed: its settings folder
// exists. Apps create it the first time they run.
func (a App) Installed() bool {
	return dirExists(filepath.Dir(a.Config))
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// Result says what Connect or Disconnect did to one app.
type Result int

const (
	Added     Result = iota // Woolly was added
	Updated                 // an existing Woolly entry was changed
	Unchanged               // the config already had this exact entry
	Removed                 // the Woolly entry was removed
	Absent                  // there was no Woolly entry to remove
)

func (r Result) String() string {
	return [...]string{"connected", "updated", "already connected", "disconnected", "not connected"}[r]
}

// ErrUnreadable means a config file exists but isn't plain JSON, for example
// because it has comments. Woolly won't rewrite a file it can't fully read.
var ErrUnreadable = errors.New("config isn't plain JSON")

// Entry builds the MCP server entry for an app's format.
func Entry(f Format, command string) map[string]any {
	if f == FormatOpenCode {
		return map[string]any{"type": "local", "command": []any{command, "mcp"}}
	}
	e := map[string]any{"command": command, "args": []any{"mcp"}}
	if f == FormatServers {
		e["type"] = "stdio"
	}
	return e
}

// path is where an app's servers live in its config, outermost key first.
func (f Format) path() []string {
	switch f {
	case FormatServers:
		return []string{"servers"}
	case FormatOpenCode:
		return []string{"mcp", "servers"}
	}
	return []string{"mcpServers"}
}

// Connect adds Woolly to an app's config, running it with command. With
// dryRun it reports what would change without writing anything.
func Connect(a App, command string, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(a.Config)
	if err != nil {
		return 0, err
	}
	servers, err := serversOf(cfg, a.Format)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", a.Config, err)
	}
	want := Entry(a.Format, command)
	res := Added
	if old, ok := servers[ServerName]; ok {
		if reflect.DeepEqual(old, want) {
			return Unchanged, nil
		}
		res = Updated
	}
	servers[ServerName] = want
	if err := setServers(cfg, a.Format, servers); err != nil {
		return 0, fmt.Errorf("%s: %w", a.Config, err)
	}
	if dryRun {
		return res, nil
	}
	return res, writeConfig(a.Config, cfg, orig)
}

// Disconnect removes Woolly from an app's config.
func Disconnect(a App, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(a.Config)
	if err != nil {
		return 0, err
	}
	if orig == nil {
		return Absent, nil
	}
	servers, err := serversOf(cfg, a.Format)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", a.Config, err)
	}
	if _, ok := servers[ServerName]; !ok {
		return Absent, nil
	}
	delete(servers, ServerName)
	if err := setServers(cfg, a.Format, servers); err != nil {
		return 0, fmt.Errorf("%s: %w", a.Config, err)
	}
	if dryRun {
		return Removed, nil
	}
	return Removed, writeConfig(a.Config, cfg, orig)
}

// IsConnected reports whether an app's config has a Woolly entry.
func IsConnected(a App) bool {
	cfg, orig, err := readConfig(a.Config)
	if err != nil || orig == nil {
		return false
	}
	servers, err := serversOf(cfg, a.Format)
	if err != nil {
		return false
	}
	_, ok := servers[ServerName]
	return ok
}

// readConfig loads a config file and also returns its bytes, which are nil
// when the file doesn't exist. A missing or empty file is an empty config.
// Numbers are kept exactly as written, so other apps' settings survive.
func readConfig(path string) (map[string]any, []byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if data == nil {
		data = []byte{}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, data, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var cfg map[string]any
	if err := dec.Decode(&cfg); err != nil || cfg == nil || dec.More() {
		return nil, data, fmt.Errorf("%s: %w", path, ErrUnreadable)
	}
	return cfg, data, nil
}

func serversOf(cfg map[string]any, f Format) (map[string]any, error) {
	path := f.path()
	cur := cfg
	for i, key := range path {
		raw, ok := cur[key]
		if !ok || raw == nil {
			return map[string]any{}, nil
		}
		next, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q should be an object: %w", strings.Join(path[:i+1], "."), ErrUnreadable)
		}
		cur = next
	}
	return cur, nil
}

// setServers puts servers back at the format's path, creating parent objects
// as needed and keeping any other keys they hold.
func setServers(cfg map[string]any, f Format, servers map[string]any) error {
	path := f.path()
	cur := cfg
	for i, key := range path[:len(path)-1] {
		raw, ok := cur[key]
		if !ok || raw == nil {
			next := map[string]any{}
			cur[key] = next
			cur = next
			continue
		}
		next, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("%q should be an object: %w", strings.Join(path[:i+1], "."), ErrUnreadable)
		}
		cur = next
	}
	cur[path[len(path)-1]] = servers
	return nil
}

// writeConfig saves the original file as a backup the first time, then
// replaces the config atomically so an app never reads a half-written file.
// orig is the file as it was read, or nil if it didn't exist.
func writeConfig(path string, cfg map[string]any, orig []byte) error {
	// Write through symlinks (common with dotfile managers) instead of
	// replacing the link with a plain file.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	mode := os.FileMode(0o644)
	if orig != nil {
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		backup := path + BackupSuffix
		if _, err := os.Stat(backup); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(backup, orig, mode); err != nil {
				return fmt.Errorf("backing up %s: %w", path, err)
			}
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".woolly-*.json")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Snippet is the JSON to paste by hand when Woolly can't edit a config.
func Snippet(f Format, command string) string {
	cfg := map[string]any{}
	_ = setServers(cfg, f, map[string]any{ServerName: Entry(f, command)})
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b)
}

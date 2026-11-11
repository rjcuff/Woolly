package connect

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", path, err, data)
	}
	return v
}

func testApp(t *testing.T, f Format) App {
	t.Helper()
	return App{ID: "test", Name: "Test", Config: filepath.Join(t.TempDir(), "app", "mcp.json"), Format: f}
}

func TestConnectCreatesConfig(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	res, err := Connect(a, "/usr/local/bin/woolly", false)
	if err != nil || res != Added {
		t.Fatalf("Connect = %v, %v; want Added", res, err)
	}
	got := readJSON(t, a.Config)
	entry := got["mcpServers"].(map[string]any)["woolly"].(map[string]any)
	if entry["command"] != "/usr/local/bin/woolly" || entry["args"].([]any)[0] != "mcp" {
		t.Fatalf("entry = %v", entry)
	}
	if _, err := os.Stat(a.Config + BackupSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("no backup should be made for a new file")
	}
	if !IsConnected(a) {
		t.Fatal("IsConnected = false after Connect")
	}
}

func TestConnectKeepsOtherSettings(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	os.MkdirAll(filepath.Dir(a.Config), 0o755)
	orig := `{"theme":"dark","mcpServers":{"github":{"command":"gh-mcp","args":[]}}}`
	os.WriteFile(a.Config, []byte(orig), 0o600)

	if _, err := Connect(a, "woolly", false); err != nil {
		t.Fatal(err)
	}
	got := readJSON(t, a.Config)
	if got["theme"] != "dark" {
		t.Fatalf("lost other settings: %v", got)
	}
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["github"]; !ok {
		t.Fatalf("lost other servers: %v", servers)
	}
	if _, ok := servers["woolly"]; !ok {
		t.Fatalf("woolly not added: %v", servers)
	}
	backup, err := os.ReadFile(a.Config + BackupSuffix)
	if err != nil || string(backup) != orig {
		t.Fatalf("backup = %q, %v; want the original file", backup, err)
	}
	if st, _ := os.Stat(a.Config); st.Mode().Perm() != 0o600 && os.PathSeparator == '/' {
		t.Fatalf("mode = %v; want the original 0600", st.Mode().Perm())
	}
}

func TestConnectTwiceIsUnchanged(t *testing.T) {
	a := testApp(t, FormatServers)
	if _, err := Connect(a, "woolly", false); err != nil {
		t.Fatal(err)
	}
	res, err := Connect(a, "woolly", false)
	if err != nil || res != Unchanged {
		t.Fatalf("second Connect = %v, %v; want Unchanged", res, err)
	}
	res, err = Connect(a, "/new/woolly", false)
	if err != nil || res != Updated {
		t.Fatalf("Connect with new path = %v, %v; want Updated", res, err)
	}
	entry := readJSON(t, a.Config)["servers"].(map[string]any)["woolly"].(map[string]any)
	if entry["type"] != "stdio" || entry["command"] != "/new/woolly" {
		t.Fatalf("VS Code entry = %v", entry)
	}
}

func TestConnectDryRunWritesNothing(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	res, err := Connect(a, "woolly", true)
	if err != nil || res != Added {
		t.Fatalf("dry run = %v, %v", res, err)
	}
	if _, err := os.Stat(a.Config); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dry run wrote the config")
	}
}

func TestConnectRefusesUnreadableConfig(t *testing.T) {
	for name, body := range map[string]string{
		"comments":      "{\n  // my servers\n  \"mcpServers\": {}\n}",
		"servers array": `{"mcpServers": []}`,
		"not an object": `[1, 2]`,
	} {
		t.Run(name, func(t *testing.T) {
			a := testApp(t, FormatMCPServers)
			os.MkdirAll(filepath.Dir(a.Config), 0o755)
			os.WriteFile(a.Config, []byte(body), 0o644)
			if _, err := Connect(a, "woolly", false); !errors.Is(err, ErrUnreadable) {
				t.Fatalf("err = %v; want ErrUnreadable", err)
			}
			if got, _ := os.ReadFile(a.Config); string(got) != body {
				t.Fatal("unreadable config was changed")
			}
		})
	}
}

func TestDisconnect(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	if res, err := Disconnect(a, false); err != nil || res != Absent {
		t.Fatalf("Disconnect with no file = %v, %v; want Absent", res, err)
	}
	os.MkdirAll(filepath.Dir(a.Config), 0o755)
	os.WriteFile(a.Config, []byte(`{"mcpServers":{"other":{"command":"x"}}}`), 0o644)
	Connect(a, "woolly", false)

	if res, err := Disconnect(a, false); err != nil || res != Removed {
		t.Fatalf("Disconnect = %v, %v; want Removed", res, err)
	}
	servers := readJSON(t, a.Config)["mcpServers"].(map[string]any)
	if _, ok := servers["woolly"]; ok {
		t.Fatal("woolly still present")
	}
	if _, ok := servers["other"]; !ok {
		t.Fatal("other server removed")
	}
	if IsConnected(a) {
		t.Fatal("IsConnected = true after Disconnect")
	}
}

func TestAppsPaths(t *testing.T) {
	tests := []struct {
		env  Env
		id   string
		want string
	}{
		{Env{GOOS: "darwin", Home: "/Users/a"}, "claude-desktop", "/Users/a/Library/Application Support/Claude/claude_desktop_config.json"},
		{Env{GOOS: "linux", Home: "/home/a"}, "vscode", "/home/a/.config/Code/User/mcp.json"},
		{Env{GOOS: "linux", Home: "/home/a"}, "cursor", "/home/a/.cursor/mcp.json"},
		{Env{GOOS: "windows", Home: `C:\Users\a`, AppData: `C:\Users\a\AppData\Roaming`}, "vscode", `C:\Users\a\AppData\Roaming\Code\User\mcp.json`},
	}
	for _, tt := range tests {
		var got string
		for _, a := range Apps(tt.env) {
			if a.ID == tt.id {
				got = a.Config
			}
		}
		// Paths are built with the OS separator, so a Windows path tested on
		// macOS has forward slashes. Compare them with either separator.
		norm := func(p string) string { return strings.ReplaceAll(p, `\`, "/") }
		if norm(got) != norm(tt.want) {
			t.Errorf("%s on %s: config = %q; want %q", tt.id, tt.env.GOOS, got, tt.want)
		}
	}
}

func TestClaudeDesktopStoreBuild(t *testing.T) {
	dir := t.TempDir()
	env := Env{GOOS: "windows", Home: dir, AppData: filepath.Join(dir, "Roaming"), LocalAppData: filepath.Join(dir, "Local")}
	store := filepath.Join(env.LocalAppData, "Packages", "Claude_abc123", "LocalCache", "Roaming", "Claude")
	os.MkdirAll(store, 0o755)

	if got, want := claudeDesktopConfig(env), filepath.Join(store, "claude_desktop_config.json"); got != want {
		t.Fatalf("Store build config = %q; want %q", got, want)
	}
	os.MkdirAll(filepath.Join(env.AppData, "Claude"), 0o755)
	if got, want := claudeDesktopConfig(env), filepath.Join(env.AppData, "Claude", "claude_desktop_config.json"); got != want {
		t.Fatalf("standard install should win: %q; want %q", got, want)
	}
}

func TestSnippet(t *testing.T) {
	var v map[string]any
	if err := json.Unmarshal([]byte(Snippet(FormatServers, "woolly")), &v); err != nil {
		t.Fatal(err)
	}
	if _, ok := v["servers"].(map[string]any)["woolly"]; !ok {
		t.Fatalf("snippet = %v", v)
	}
}

func TestConnectPreservesNumbersAndText(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	os.MkdirAll(filepath.Dir(a.Config), 0o755)
	os.WriteFile(a.Config, []byte(`{"id": 9007199254740993, "ratio": 0.1, "note": "a <b> & c", "mcpServers": {}}`), 0o644)
	if _, err := Connect(a, "woolly", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(a.Config)
	for _, want := range []string{"9007199254740993", "0.1", `"a <b> & c"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("config lost %s:\n%s", want, data)
		}
	}
}

func TestBackupKeepsTheOriginal(t *testing.T) {
	a := testApp(t, FormatMCPServers)
	os.MkdirAll(filepath.Dir(a.Config), 0o755)
	orig := `{"mcpServers":{"x":{"command":"x"}}}`
	os.WriteFile(a.Config, []byte(orig), 0o644)
	Connect(a, "woolly", false)
	Disconnect(a, false)
	Connect(a, "other", false)
	if got, _ := os.ReadFile(a.Config + BackupSuffix); string(got) != orig {
		t.Fatalf("backup = %s; want the original %s", got, orig)
	}
}

func TestConnectFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "mcp.json")
	os.MkdirAll(filepath.Dir(real), 0o755)
	os.WriteFile(real, []byte(`{}`), 0o644)
	link := filepath.Join(dir, "app", "mcp.json")
	os.MkdirAll(filepath.Dir(link), 0o755)
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks not available:", err)
	}
	a := App{Config: link, Format: FormatMCPServers}
	if _, err := Connect(a, "woolly", false); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced by a file")
	}
	if !IsConnected(App{Config: real, Format: FormatMCPServers}) {
		t.Fatal("target file not updated")
	}
}

func TestOpenCodeNestedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	os.WriteFile(path, []byte(`{"theme":"dark","mcp":{"timeout":5000,"servers":{"other":{"type":"local","command":["x"]}}}}`), 0o644)
	a := App{ID: "opencode", Name: "opencode", Config: path, Format: FormatOpenCode}

	if res, err := Connect(a, "/bin/woolly", false); err != nil || res != Added {
		t.Fatalf("connect: %v %v", res, err)
	}
	var cfg map[string]any
	data, _ := os.ReadFile(path)
	json.Unmarshal(data, &cfg)
	mcp := cfg["mcp"].(map[string]any)
	servers := mcp["servers"].(map[string]any)
	w := servers["woolly"].(map[string]any)
	if cfg["theme"] != "dark" || mcp["timeout"].(float64) != 5000 || servers["other"] == nil {
		t.Fatalf("other settings lost: %s", data)
	}
	if w["type"] != "local" || w["command"].([]any)[0] != "/bin/woolly" || w["command"].([]any)[1] != "mcp" {
		t.Fatalf("entry: %v", w)
	}
	if !IsConnected(a) {
		t.Fatal("should report connected")
	}
	if res, _ := Disconnect(a, false); res != Removed || IsConnected(a) {
		t.Fatal("disconnect failed")
	}
}

func TestOpenCodeFromEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	a := App{Config: path, Format: FormatOpenCode}
	if _, err := Connect(a, "/bin/woolly", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"servers"`) || !strings.Contains(string(data), `"mcp"`) {
		t.Fatalf("got %s", data)
	}
}

func TestNewAppsPaths(t *testing.T) {
	home := t.TempDir()
	env := Env{GOOS: "darwin", Home: home}
	want := map[string]string{
		"gemini":   filepath.Join(home, ".gemini", "settings.json"),
		"lmstudio": filepath.Join(home, ".lmstudio", "mcp.json"),
		"opencode": filepath.Join(home, ".config", "opencode", "opencode.json"),
	}
	for _, a := range Apps(env) {
		if p, ok := want[a.ID]; ok && a.Config != p {
			t.Errorf("%s: %s, want %s", a.ID, a.Config, p)
		}
	}
	os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755)
	os.WriteFile(filepath.Join(home, ".config", "opencode", "opencode.jsonc"), []byte("{}"), 0o644)
	for _, a := range Apps(env) {
		if a.ID == "opencode" && !strings.HasSuffix(a.Config, "opencode.jsonc") {
			t.Errorf("should prefer opencode.jsonc when it exists, got %s", a.Config)
		}
	}
}

func TestGeminiKeepsOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"theme":"GitHub","selectedAuthType":"oauth-personal"}`), 0o644)
	a := App{Config: path, Format: FormatMCPServers}
	if _, err := Connect(a, "/bin/woolly", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, s := range []string{`"theme": "GitHub"`, `"selectedAuthType": "oauth-personal"`, `"woolly"`} {
		if !strings.Contains(string(data), s) {
			t.Fatalf("missing %s in %s", s, data)
		}
	}
}

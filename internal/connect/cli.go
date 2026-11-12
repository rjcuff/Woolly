package connect

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CLIApp is an app set up through its own command, like `claude mcp add`.
type CLIApp struct {
	ID, Name, Bin string
}

// Claude Code and Codex manage their own MCP settings.
var (
	ClaudeCode = CLIApp{ID: "claude-code", Name: "Claude Code", Bin: "claude"}
	Codex      = CLIApp{ID: "codex", Name: "Codex", Bin: "codex"}
)

// LookPath finds a program on PATH, then in the folders installers use,
// since apps opened from the Dock don't get the terminal's PATH.
func LookPath(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".claude", "local"),
		filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".volta", "bin"),
		filepath.Join(home, "go", "bin"),
		"/opt/homebrew/bin",
		"/usr/local/bin",
	}
	if runtime.GOOS == "windows" {
		name += ".exe"
		dirs = append(dirs, filepath.Join(os.Getenv("APPDATA"), "npm"))
	}
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", exec.ErrNotFound
}

// Installed reports whether the app's command can be found.
func (c CLIApp) Installed() bool {
	_, err := LookPath(c.Bin)
	return err == nil
}

// Connected reports whether the app already has Woolly.
func (c CLIApp) Connected() bool {
	if c.ID == Codex.ID {
		return codexHasWoolly()
	}
	bin, err := LookPath(c.Bin)
	if err != nil {
		return false
	}
	return exec.Command(bin, "mcp", "get", ServerName).Run() == nil
}

// Connect adds Woolly through the app's command. For Claude Code it also
// adds the startup hook.
func (c CLIApp) Connect(command string, dryRun bool) (Result, error) {
	bin, err := LookPath(c.Bin)
	if err != nil {
		return 0, fmt.Errorf("the %s command isn't installed", c.Bin)
	}
	res := Unchanged
	if !c.Connected() {
		res = Added
		if !dryRun {
			args := []string{"mcp", "add", ServerName, "--", command, "mcp"}
			if c.ID == ClaudeCode.ID {
				args = []string{"mcp", "add", "--scope", "user", ServerName, "--", command, "mcp"}
			}
			if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
				return 0, fmt.Errorf("%s mcp add failed: %s", c.Bin, strings.TrimSpace(string(out)))
			}
		}
	}
	return res, nil
}

// Disconnect removes Woolly through the app's command.
func (c CLIApp) Disconnect(dryRun bool) (Result, error) {
	if !c.Connected() {
		return Absent, nil
	}
	if dryRun {
		return Removed, nil
	}
	bin, err := LookPath(c.Bin)
	if err != nil {
		return 0, err
	}
	args := []string{"mcp", "remove", ServerName}
	if c.ID == ClaudeCode.ID {
		args = append(args, "--scope", "user")
	}
	if out, err := exec.Command(bin, args...).CombinedOutput(); err != nil {
		msg := fmt.Sprintf("%s mcp remove failed: %s", c.Bin, strings.TrimSpace(string(out)))
		if c.ID == Codex.ID {
			msg += fmt.Sprintf(". Remove the [mcp_servers.%s] section from %s by hand", ServerName, CodexConfig())
		}
		return 0, errors.New(msg)
	}
	return Removed, nil
}

// CodexConfig is Codex's config file.
func CodexConfig() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return filepath.Join(dir, "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func codexHasWoolly() bool {
	data, err := os.ReadFile(CodexConfig())
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
		if l == "[mcp_servers."+ServerName+"]" || l == `[mcp_servers."`+ServerName+`"]` {
			return true
		}
	}
	return false
}

// WoollyCommand is the command apps should run to start Woolly. It prefers
// `woolly` on PATH, which survives upgrades, over this program's own path.
func WoollyCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding the woolly binary: %w", err)
	}
	if p, err := LookPath("woolly"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs, nil
		}
	}
	return exe, nil
}

// Status is one app's state, for the app's Apps screen.
type Status struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Connected bool   `json:"connected"`
	Restart   bool   `json:"restart,omitempty"`
}

// Statuses lists every app Woolly can connect to.
func Statuses(e Env) []Status {
	var out []Status
	for _, c := range []CLIApp{ClaudeCode, Codex} {
		installed := c.Installed()
		out = append(out, Status{ID: c.ID, Name: c.Name, Installed: installed, Connected: installed && c.Connected()})
	}
	for _, a := range Apps(e) {
		out = append(out, Status{ID: a.ID, Name: a.Name, Installed: a.Installed(), Connected: IsConnected(a), Restart: a.Restart})
	}
	return out
}

// ConnectByID connects or disconnects one app.
func ConnectByID(e Env, id, command string, remove bool) (Result, error) {
	for _, c := range []CLIApp{ClaudeCode, Codex} {
		if c.ID != id {
			continue
		}
		var res Result
		var err error
		if remove {
			res, err = c.Disconnect(false)
		} else {
			res, err = c.Connect(command, false)
		}
		if err == nil && c.ID == ClaudeCode.ID {
			home, _ := os.UserHomeDir()
			if remove {
				_, err = RemoveStartupHook(ClaudeSettings(home), false)
			} else {
				_, err = AddStartupHook(ClaudeSettings(home), command, false)
			}
		}
		return res, err
	}
	for _, a := range Apps(e) {
		if a.ID != id {
			continue
		}
		if remove {
			return Disconnect(a, false)
		}
		return Connect(a, command, false)
	}
	return 0, fmt.Errorf("unknown app %q", id)
}

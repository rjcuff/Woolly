// Package service starts Woolly at login: a LaunchAgent on macOS, a systemd
// user unit on Linux, and a Run key on Windows. None need admin rights.
package service

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Label names the background service on every OS.
const Label = "com.usewoolly.woolly"

// winRunKey is where Windows looks for programs to start at login.
const winRunKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

const winRunValue = "Woolly"

// ErrUnsupported means this OS has no service mechanism Woolly can use.
var ErrUnsupported = errors.New("running in the background isn't supported on this system")

// Spec describes the background process.
type Spec struct {
	Exe  string            // absolute path to the woolly binary
	Args []string          // arguments, e.g. serve --proxy
	Log  string            // file that receives output
	Env  map[string]string // environment to pass through, e.g. WOOLLY_HOME
	Home string            // user home directory
}

// Runner runs a command and returns its combined output. Tests replace it.
type Runner func(name string, args ...string) ([]byte, error)

// ExecRunner runs real commands.
func ExecRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// Manager installs and removes the service for one OS.
type Manager struct {
	GOOS string
	Run  Runner
	UID  int // used by launchctl on macOS
	// RetryDelay is the pause between launchctl bootstrap attempts.
	RetryDelay time.Duration
}

func (m *Manager) retryDelay() time.Duration {
	if m.RetryDelay > 0 {
		return m.RetryDelay
	}
	return 500 * time.Millisecond
}

// New returns a Manager for the running system.
func New() *Manager {
	return &Manager{GOOS: runtime.GOOS, Run: ExecRunner, UID: os.Getuid()}
}

// PlistPath is where the macOS LaunchAgent lives.
func PlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist")
}

// UnitPath is where the systemd user unit lives.
func UnitPath(home string) string {
	return filepath.Join(home, ".config", "systemd", "user", "woolly.service")
}

// Plist renders the macOS LaunchAgent.
func Plist(s Spec) string {
	var b strings.Builder
	esc := func(v string) string {
		var out bytes.Buffer
		_ = xml.EscapeText(&out, []byte(v))
		return out.String()
	}
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>` + Label + `</string>
  <key>ProgramArguments</key>
  <array>
`)
	for _, a := range append([]string{s.Exe}, s.Args...) {
		b.WriteString("    <string>" + esc(a) + "</string>\n")
	}
	b.WriteString("  </array>\n")
	if len(s.Env) > 0 {
		b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
		for _, k := range sortedKeys(s.Env) {
			b.WriteString("    <key>" + esc(k) + "</key>\n    <string>" + esc(s.Env[k]) + "</string>\n")
		}
		b.WriteString("  </dict>\n")
	}
	b.WriteString(`  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
  <key>ProcessType</key>
  <string>Background</string>
  <key>StandardOutPath</key>
  <string>` + esc(s.Log) + `</string>
  <key>StandardErrorPath</key>
  <string>` + esc(s.Log) + `</string>
</dict>
</plist>
`)
	return b.String()
}

// Unit renders the systemd user unit.
func Unit(s Spec) string {
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=Woolly, local memory for your AI tools\nAfter=network.target\n\n[Service]\n")
	words := make([]string, 0, len(s.Args)+1)
	for _, a := range append([]string{s.Exe}, s.Args...) {
		words = append(words, systemdQuote(a))
	}
	b.WriteString("ExecStart=" + strings.Join(words, " ") + "\n")
	for _, k := range sortedKeys(s.Env) {
		b.WriteString("Environment=" + systemdQuote(k+"="+s.Env[k]) + "\n")
	}
	b.WriteString("Restart=on-failure\nRestartSec=5\n\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

// systemdQuote quotes one word for a unit file. % starts a specifier there,
// so it is doubled.
func systemdQuote(s string) string {
	s = strings.ReplaceAll(s, "%", "%%")
	if s != "" && !strings.ContainsAny(s, " \t\"'\\$") {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `$$`)
	return `"` + r.Replace(s) + `"`
}

// WindowsCommand is the command line stored in the Run key. The output goes
// to the log file through the --log flag, since there is no console.
func WindowsCommand(s Spec) string {
	parts := []string{"conhost.exe", "--headless", winQuote(s.Exe)}
	for _, a := range s.Args {
		parts = append(parts, winQuote(a))
	}
	return strings.Join(parts, " ")
}

func winQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Install registers the service and starts it now.
func (m *Manager) Install(s Spec) error {
	if err := os.MkdirAll(filepath.Dir(s.Log), 0o755); err != nil {
		return fmt.Errorf("creating the log folder: %w", err)
	}
	switch m.GOOS {
	case "darwin":
		return m.installLaunchd(s)
	case "linux":
		return m.installSystemd(s)
	case "windows":
		return m.installWindows(s)
	}
	return ErrUnsupported
}

// Uninstall stops the service and stops it starting at login. On Windows the
// running process is stopped by the caller, which knows its PID.
func (m *Manager) Uninstall(home string) error {
	switch m.GOOS {
	case "darwin":
		p := PlistPath(home)
		_, _ = m.Run("launchctl", "bootout", m.domain()+"/"+Label)
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", p, err)
		}
		return nil
	case "linux":
		_, _ = m.Run("systemctl", "--user", "disable", "--now", "woolly.service")
		p := UnitPath(home)
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", p, err)
		}
		_, _ = m.Run("systemctl", "--user", "daemon-reload")
		return nil
	case "windows":
		if !m.Installed(home) {
			return nil
		}
		if out, err := m.Run("reg", "delete", winRunKey, "/v", winRunValue, "/f"); err != nil {
			return fmt.Errorf("removing the startup entry: %s: %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	return ErrUnsupported
}

// Installed reports whether the service is registered to start at login.
func (m *Manager) Installed(home string) bool {
	switch m.GOOS {
	case "darwin":
		return fileExists(PlistPath(home))
	case "linux":
		return fileExists(UnitPath(home))
	case "windows":
		_, err := m.Run("reg", "query", winRunKey, "/v", winRunValue)
		return err == nil
	}
	return false
}

func (m *Manager) domain() string { return "gui/" + strconv.Itoa(m.UID) }

func (m *Manager) installLaunchd(s Spec) error {
	p := PlistPath(s.Home)
	if err := writeFile(p, Plist(s)); err != nil {
		return err
	}
	// bootout fails when nothing is loaded yet; that's fine. It also returns
	// before the old job is gone, so bootstrap is retried for a few seconds.
	_, _ = m.Run("launchctl", "bootout", m.domain()+"/"+Label)
	var out []byte
	var err error
	for i := 0; i < 10; i++ {
		if out, err = m.Run("launchctl", "bootstrap", m.domain(), p); err == nil {
			return nil
		}
		time.Sleep(m.retryDelay())
	}
	return fmt.Errorf("launchctl bootstrap: %s: %w", strings.TrimSpace(string(out)), err)
}

func (m *Manager) installSystemd(s Spec) error {
	if _, err := m.Run("systemctl", "--user", "--version"); err != nil {
		return fmt.Errorf("systemd user services aren't available here, so run `woolly serve --proxy` yourself: %w", ErrUnsupported)
	}
	if err := writeFile(UnitPath(s.Home), Unit(s)); err != nil {
		return err
	}
	if out, err := m.Run("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := m.Run("systemctl", "--user", "enable", "--now", "woolly.service"); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", strings.TrimSpace(string(out)), err)
	}
	// Restart picks up a changed unit when the service was already running.
	_, _ = m.Run("systemctl", "--user", "restart", "woolly.service")
	return nil
}

// installWindows adds the Run entry. The caller starts the process now, since
// the Run key only takes effect at the next login.
func (m *Manager) installWindows(s Spec) error {
	cmd := WindowsCommand(s)
	if out, err := m.Run("reg", "add", winRunKey, "/v", winRunValue, "/t", "REG_SZ", "/d", cmd, "/f"); err != nil {
		return fmt.Errorf("adding the startup entry: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// StartsItself reports whether Install also starts the process. On Windows
// the caller must start it.
func (m *Manager) StartsItself() bool { return m.GOOS != "windows" }

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

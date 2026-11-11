package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/proxy"
	"github.com/rjcuff/woolly/internal/server"
	"github.com/rjcuff/woolly/internal/service"
)

// maxLogSize is how big the background log may grow before it is rotated.
const maxLogSize = 10 << 20

// passEnv are the settings the background process should keep. launchd and
// systemd start it with a bare environment.
var passEnv = []string{"WOOLLY_HOME", "WOOLLY_HOST", "WOOLLY_PROXY_HOST", "WOOLLY_NO_EMBED", "OLLAMA_HOST", "OLLAMA_EMBED_MODEL"}

const serviceUsage = `Usage:
  woolly service install     run Woolly in the background and start it at login
  woolly service uninstall   stop it and remove it from login
  woolly service status      show whether it's running
`

func cmdService(args []string) error {
	if len(args) == 0 {
		fmt.Print(serviceUsage)
		return nil
	}
	switch args[0] {
	case "install", "start", "enable":
		return serviceInstall()
	case "uninstall", "stop", "disable", "remove":
		return serviceUninstall()
	case "status":
		return serviceStatus()
	case "help", "-h", "--help":
		fmt.Print(serviceUsage)
		return nil
	}
	return fmt.Errorf("unknown service command %q\n%s", args[0], serviceUsage)
}

func serviceSpec() (service.Spec, string, error) {
	h, err := home()
	if err != nil {
		return service.Spec{}, "", err
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return service.Spec{}, "", fmt.Errorf("finding your home folder: %w", err)
	}
	exe, err := woollyCommand()
	if err != nil {
		return service.Spec{}, "", err
	}
	logPath := filepath.Join(h, "logs", "woolly.log")
	env := map[string]string{}
	for _, k := range passEnv {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			env[k] = v
		}
	}
	args := []string{"serve", "--proxy", "--log", logPath}
	if runtime.GOOS == "windows" {
		// A Run key entry has no environment of its own, so pass the
		// settings as flags to keep them after the next login.
		for _, k := range passEnv {
			if v, ok := env[k]; ok {
				args = append(args, "--env", k+"="+v)
			}
		}
	}
	return service.Spec{
		Exe:  exe,
		Args: args,
		Log:  logPath,
		Env:  env,
		Home: user,
	}, h, nil
}

func serviceInstall() error {
	spec, h, err := serviceSpec()
	if err != nil {
		return err
	}
	m := service.New()
	apiAddr := envOr("WOOLLY_HOST", server.DefaultAddr)
	wasRunning := listening(apiAddr)
	if wasRunning && !m.Installed(spec.Home) {
		return fmt.Errorf("something is already using %s. If it's `woolly serve` or `woolly proxy`, stop it first, then run this again", apiAddr)
	}
	if err := m.Install(spec); err != nil {
		return err
	}
	if !m.StartsItself() {
		if wasRunning {
			stopPID(h)
			waitFor(func() bool { return !listening(apiAddr) }, 3*time.Second)
		}
		if err := startDetached(spec.Exe, spec.Args); err != nil {
			return fmt.Errorf("starting Woolly: %w", err)
		}
	}
	if !waitFor(func() bool { return apiUp(apiAddr) }, 8*time.Second) {
		return fmt.Errorf("Woolly is set to start at login, but it didn't start now. See the log: %s", spec.Log)
	}
	fmt.Printf(`Woolly is running in the background and will start when you log in.

  Review page:  woolly ui
  API:          http://%s
  Proxy:        http://%s  (OLLAMA_HOST=%s ollama run llama3.2)
  Log:          %s

Stop it with: woolly service uninstall
`, apiAddr, proxyAddr(), proxyAddr(), spec.Log)
	return nil
}

func serviceUninstall() error {
	spec, h, err := serviceSpec()
	if err != nil {
		return err
	}
	m := service.New()
	installed := m.Installed(spec.Home)
	if err := m.Uninstall(spec.Home); err != nil {
		return err
	}
	if !m.StartsItself() && installed {
		stopPID(h)
	}
	if !installed {
		fmt.Println("Woolly wasn't set up to run in the background.")
		return nil
	}
	fmt.Println("Stopped Woolly and removed it from login. Start it again with: woolly service install")
	return nil
}

func serviceStatus() error {
	spec, _, err := serviceSpec()
	if err != nil {
		return err
	}
	fmt.Println(serviceSummary(spec))
	return nil
}

// serviceSummary describes the background service in a few lines, for
// woolly service status and woolly doctor.
func serviceSummary(spec service.Spec) string {
	m := service.New()
	apiAddr := envOr("WOOLLY_HOST", server.DefaultAddr)
	installed := "no (run `woolly service install`)"
	if m.Installed(spec.Home) {
		installed = "yes"
	}
	return fmt.Sprintf("  starts at login: %s\n  api %s: %s\n  proxy %s: %s\n  log: %s",
		installed, apiAddr, upDown(apiUp(apiAddr)), proxyAddr(), upDown(listening(proxyAddr())), spec.Log)
}

func upDown(up bool) string {
	if up {
		return "running"
	}
	return "not running"
}

func proxyAddr() string { return envOr("WOOLLY_PROXY_HOST", proxy.DefaultAddr) }

func listening(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func apiUp(addr string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	res, err := client.Get("http://" + addr + "/api/version")
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode < 500
}

func waitFor(ok func() bool, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if ok() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// redirectOutput sends everything the process prints to a log file, rotating
// it once it grows past maxLogSize.
func redirectOutput(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating the log folder: %w", err)
	}
	if st, err := os.Stat(path); err == nil && st.Size() > maxLogSize {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening the log: %w", err)
	}
	os.Stdout, os.Stderr = f, f
	log.SetOutput(io.Writer(f))
	fmt.Fprintf(f, "\n--- %s ---\n", time.Now().Format(time.RFC3339))
	return nil
}

func pidPath(h string) string { return filepath.Join(h, "woolly.pid") }

// writePID records the server's process ID so it can be stopped later.
func writePID(h string) error {
	if err := os.MkdirAll(h, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", h, err)
	}
	if err := os.WriteFile(pidPath(h), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("writing the PID file: %w", err)
	}
	return nil
}

// removePID deletes the PID file if it still belongs to this process.
func removePID(h string) {
	if pid, err := readPID(h); err == nil && pid == os.Getpid() {
		_ = os.Remove(pidPath(h))
	}
}

func readPID(h string) (int, error) {
	data, err := os.ReadFile(pidPath(h))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, errors.New("bad PID file")
	}
	return pid, nil
}

// stopPID stops the server recorded in the PID file. The file can be stale
// after a crash or reboot, and the PID reused, so it only stops a process
// that is actually woolly.
func stopPID(h string) {
	pid, err := readPID(h)
	if err != nil {
		return
	}
	if processIsWoolly(pid) {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
	_ = os.Remove(pidPath(h))
}

// processIsWoolly reports whether pid is a running woolly process.
func processIsWoolly(pid int) bool {
	var out []byte
	var err error
	if runtime.GOOS == "windows" {
		out, err = exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	} else {
		out, err = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	}
	return err == nil && strings.Contains(strings.ToLower(string(out)), "woolly")
}

// envFlags collects --env KEY=VALUE settings for woolly serve.
type envFlags []string

func (e *envFlags) String() string { return strings.Join(*e, ",") }

func (e *envFlags) Set(v string) error {
	k, _, ok := strings.Cut(v, "=")
	if !ok || !slices.Contains(passEnv, k) {
		return fmt.Errorf("--env takes KEY=VALUE with one of %s", strings.Join(passEnv, ", "))
	}
	*e = append(*e, v)
	return nil
}

func (e envFlags) apply() error {
	for _, kv := range e {
		k, v, _ := strings.Cut(kv, "=")
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("setting %s: %w", k, err)
		}
	}
	return nil
}

// flagSet reports whether a flag was given on the command line.
func flagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

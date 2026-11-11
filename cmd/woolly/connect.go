package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rjcuff/woolly/internal/connect"
)

// Claude Code and Codex are set up through their own CLIs rather than by
// editing a file.
const (
	claudeCodeID = "claude-code"
	codexID      = "codex"
)

// woollyCommand is the command apps should run to start Woolly. It prefers the
// path on PATH (like /opt/homebrew/bin/woolly), which survives upgrades,
// over the versioned path the running binary may have been resolved to.
func woollyCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding the woolly binary: %w", err)
	}
	if onPath, err := exec.LookPath("woolly"); err == nil {
		if abs, err := filepath.Abs(onPath); err == nil && sameFile(abs, exe) {
			return abs, nil
		}
	}
	return exe, nil
}

func sameFile(a, b string) bool {
	sa, err1 := os.Stat(a)
	sb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(sa, sb)
}

func cmdConnect(args []string) error {
	fs := newFlags("connect")
	dryRun := fs.Bool("dry-run", false, "show what would change without changing anything")
	remove := fs.Bool("remove", false, "remove Woolly from the apps instead")
	ids, err := parse(fs, args)
	if err != nil {
		return err
	}
	env, err := connect.EnvFromOS(runtime.GOOS)
	if err != nil {
		return err
	}
	apps := connect.Apps(env)
	wanted, err := pickApps(apps, ids)
	if err != nil {
		return err
	}
	command, err := woollyCommand()
	if err != nil {
		return err
	}

	verb := "Connecting"
	if *remove {
		verb = "Disconnecting"
	}
	fmt.Printf("%s Woolly (%s)\n", verb, command)
	if *dryRun {
		fmt.Println("Dry run: nothing will change.")
	}
	fmt.Println()

	var restart []string
	done := 0
	if wanted[claudeCodeID] {
		if claudeCode(command, *remove, *dryRun, len(ids) > 0) {
			done++
		}
	}
	if wanted[codexID] {
		if codex(command, *remove, *dryRun, len(ids) > 0) {
			done++
		}
	}
	for _, a := range apps {
		if !wanted[a.ID] {
			continue
		}
		if len(ids) == 0 && !a.Installed() {
			printRow("-", a.Name, "not found")
			continue
		}
		res, err := applyApp(a, command, *remove, *dryRun)
		if err != nil {
			printRow("!", a.Name, explain(err, a, command))
			continue
		}
		printRow("✓", a.Name, describe(res, *dryRun))
		done++
		if a.Restart && (res == connect.Added || res == connect.Updated || res == connect.Removed) {
			restart = append(restart, a.Name)
		}
	}
	fmt.Println()
	switch {
	case done == 0 && !*remove:
		fmt.Println("No apps found. Install one, or see https://usewoolly.com/docs/connect to connect any MCP app.")
	case len(restart) > 0 && !*dryRun:
		fmt.Printf("Restart %s to load the change.\n", strings.Join(restart, " and "))
	}
	return nil
}

// pickApps turns app names from the command line into a set. No names means
// every app.
func pickApps(apps []connect.App, ids []string) (map[string]bool, error) {
	known := map[string]bool{claudeCodeID: true, codexID: true}
	names := []string{claudeCodeID, codexID}
	for _, a := range apps {
		known[a.ID] = true
		names = append(names, a.ID)
	}
	if len(ids) == 0 {
		return known, nil
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if !known[id] {
			return nil, fmt.Errorf("unknown app %q (choose from: %s)", id, strings.Join(names, ", "))
		}
		wanted[id] = true
	}
	return wanted, nil
}

func applyApp(a connect.App, command string, remove, dryRun bool) (connect.Result, error) {
	if remove {
		return connect.Disconnect(a, dryRun)
	}
	return connect.Connect(a, command, dryRun)
}

func explain(err error, a connect.App, command string) string {
	if errors.Is(err, connect.ErrUnreadable) {
		return fmt.Sprintf("can't safely edit %s (it has comments or an unexpected layout).\n      Add this to it yourself:\n%s",
			a.Config, indent(connect.Snippet(a.Format, command), "      "))
	}
	return err.Error()
}

// describe words a result, as a plan in a dry run.
func describe(r connect.Result, dryRun bool) string {
	if !dryRun {
		return r.String()
	}
	switch r {
	case connect.Added:
		return "would connect"
	case connect.Updated:
		return "would update"
	case connect.Removed:
		return "would disconnect"
	}
	return r.String()
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

func printRow(mark, name, msg string) {
	fmt.Printf("  %s %-15s %s\n", mark, name, msg)
}

// claudeCode connects Claude Code through its own CLI, for every project,
// and adds a startup hook so each session begins with what Woolly knows.
// It reports whether it did anything.
func claudeCode(command string, remove, dryRun, explicit bool) bool {
	const name = "Claude Code"
	claude, err := exec.LookPath("claude")
	if err != nil {
		if explicit {
			printRow("!", name, "the claude command isn't installed")
		} else {
			printRow("-", name, "not found")
		}
		return false
	}
	connected := exec.Command(claude, "mcp", "get", connect.ServerName).Run() == nil
	res := connect.Unchanged
	switch {
	case remove && !connected:
		res = connect.Absent
	case !remove && connected:
	case dryRun && remove:
		res = connect.Removed
	case dryRun:
		res = connect.Added
	default:
		var cmd *exec.Cmd
		if remove {
			cmd = exec.Command(claude, "mcp", "remove", connect.ServerName, "--scope", "user")
			res = connect.Removed
		} else {
			cmd = exec.Command(claude, "mcp", "add", "--scope", "user", connect.ServerName, "--", command, "mcp")
			res = connect.Added
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			printRow("!", name, fmt.Sprintf("claude mcp failed: %s", strings.TrimSpace(string(out))))
			return false
		}
	}
	printRow("✓", name, describe(res, dryRun))
	startupHook(command, remove, dryRun)
	return true
}

// startupHook adds or removes the Claude Code hook that runs `woolly context`
// when a session starts.
func startupHook(command string, remove, dryRun bool) {
	const name = "  at startup"
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := connect.ClaudeSettings(home)
	var res connect.Result
	if remove {
		res, err = connect.RemoveStartupHook(path, dryRun)
	} else {
		res, err = connect.AddStartupHook(path, command, dryRun)
	}
	switch {
	case errors.Is(err, connect.ErrUnreadable):
		printRow("!", name, fmt.Sprintf("can't safely edit %s. To load your profile at startup, add a SessionStart hook that runs:\n      %s", path, connect.HookCommand(command)))
	case err != nil:
		printRow("!", name, err.Error())
	case remove && res == connect.Absent:
		printRow("✓", name, "no startup hook")
	case remove && dryRun:
		printRow("✓", name, "would remove the startup hook")
	case remove:
		printRow("✓", name, "startup hook removed")
	default:
		msg := "loads your profile when a session starts"
		if dryRun && res != connect.Unchanged {
			msg = "would load your profile when a session starts"
		}
		printRow("✓", name, msg)
	}
}

// codex connects the Codex CLI through `codex mcp add`. Codex keeps servers
// in ~/.codex/config.toml (or $CODEX_HOME), which is read to see whether
// Woolly is already there.
func codex(command string, remove, dryRun, explicit bool) bool {
	const name = "Codex"
	bin, err := exec.LookPath("codex")
	if err != nil {
		if explicit {
			printRow("!", name, "the codex command isn't installed")
		} else {
			printRow("-", name, "not found")
		}
		return false
	}
	connected := codexHasWoolly()
	switch {
	case remove && !connected:
		printRow("✓", name, connect.Absent.String())
		return true
	case !remove && connected:
		printRow("✓", name, connect.Unchanged.String())
		return true
	case dryRun && remove:
		printRow("✓", name, describe(connect.Removed, true))
		return true
	case dryRun:
		printRow("✓", name, describe(connect.Added, true))
		return true
	}
	var cmd *exec.Cmd
	if remove {
		cmd = exec.Command(bin, "mcp", "remove", connect.ServerName)
	} else {
		cmd = exec.Command(bin, "mcp", "add", connect.ServerName, "--", command, "mcp")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := fmt.Sprintf("codex mcp failed: %s", strings.TrimSpace(string(out)))
		if remove {
			msg += fmt.Sprintf(". Remove the [mcp_servers.%s] section from %s by hand", connect.ServerName, codexConfig())
		}
		printRow("!", name, msg)
		return false
	}
	if remove {
		printRow("✓", name, connect.Removed.String())
	} else {
		printRow("✓", name, connect.Added.String())
	}
	return true
}

func codexConfig() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return filepath.Join(dir, "config.toml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func codexHasWoolly() bool {
	data, err := os.ReadFile(codexConfig())
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
		if l == "[mcp_servers."+connect.ServerName+"]" || l == `[mcp_servers."`+connect.ServerName+`"]` {
			return true
		}
	}
	return false
}

// connectedApps lists the installed apps that have Woolly, for woolly doctor.
func connectedApps() (connected, notConnected []string) {
	env, err := connect.EnvFromOS(runtime.GOOS)
	if err != nil {
		return nil, nil
	}
	for _, a := range connect.Apps(env) {
		if !a.Installed() {
			continue
		}
		if connect.IsConnected(a) {
			connected = append(connected, a.Name)
		} else {
			notConnected = append(notConnected, a.Name)
		}
	}
	return connected, notConnected
}

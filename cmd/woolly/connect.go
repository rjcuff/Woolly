package main

import (
	"errors"
	"fmt"
	"os"
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

func woollyCommand() (string, error) { return connect.WoollyCommand() }

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

// claudeCode connects Claude Code and adds its startup hook. It reports
// whether it did anything.
func claudeCode(command string, remove, dryRun, explicit bool) bool {
	if !cliApp(connect.ClaudeCode, command, remove, dryRun, explicit) {
		return false
	}
	startupHook(command, remove, dryRun)
	return true
}

func cliApp(c connect.CLIApp, command string, remove, dryRun, explicit bool) bool {
	if !c.Installed() {
		if explicit {
			printRow("!", c.Name, "the "+c.Bin+" command isn't installed")
		} else {
			printRow("-", c.Name, "not found")
		}
		return false
	}
	var res connect.Result
	var err error
	if remove {
		res, err = c.Disconnect(dryRun)
	} else {
		res, err = c.Connect(command, dryRun)
	}
	if err != nil {
		printRow("!", c.Name, err.Error())
		return false
	}
	printRow("✓", c.Name, describe(res, dryRun))
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

func codex(command string, remove, dryRun, explicit bool) bool {
	return cliApp(connect.Codex, command, remove, dryRun, explicit)
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

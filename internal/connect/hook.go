package connect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookApp is the name the startup hook gives itself in the activity log.
const HookApp = "claude-code"

// hookArgs ends every hook command Woolly adds, so it can find its own.
const hookArgs = "context --app " + HookApp

// hookMatcher skips resumed sessions, which already have the context.
const hookMatcher = "startup|clear|compact"

// ClaudeSettings is Claude Code's user settings file.
func ClaudeSettings(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// HookCommand is the command the startup hook runs.
func HookCommand(woolly string) string {
	if strings.ContainsAny(woolly, " \t'\"") {
		woolly = `"` + strings.ReplaceAll(woolly, `"`, `\"`) + `"`
	}
	return woolly + " " + hookArgs
}

func isOurs(h any) bool {
	m, ok := h.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.HasSuffix(strings.TrimSpace(cmd), hookArgs) && strings.Contains(strings.ToLower(cmd), "woolly")
}

// sessionStart returns settings.hooks.SessionStart, checking its shape.
func sessionStart(cfg map[string]any) (map[string]any, []any, error) {
	hooks := map[string]any{}
	if raw, ok := cfg["hooks"]; ok && raw != nil {
		h, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf(`"hooks" should be an object: %w`, ErrUnreadable)
		}
		hooks = h
	}
	var groups []any
	if raw, ok := hooks["SessionStart"]; ok && raw != nil {
		g, ok := raw.([]any)
		if !ok {
			return nil, nil, fmt.Errorf(`"hooks.SessionStart" should be a list: %w`, ErrUnreadable)
		}
		groups = g
	}
	return hooks, groups, nil
}

// findHook returns the group and hook index of Woolly's hook, or -1s.
func findHook(groups []any) (int, int) {
	for gi, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		list, _ := gm["hooks"].([]any)
		for hi, h := range list {
			if isOurs(h) {
				return gi, hi
			}
		}
	}
	return -1, -1
}

// HasStartupHook reports whether Claude Code's settings load Woolly at startup.
func HasStartupHook(path string) bool {
	cfg, _, err := readConfig(path)
	if err != nil {
		return false
	}
	_, groups, err := sessionStart(cfg)
	if err != nil {
		return false
	}
	gi, _ := findHook(groups)
	return gi >= 0
}

// AddStartupHook adds a SessionStart hook that runs `woolly context`.
func AddStartupHook(path, woolly string, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(path)
	if err != nil {
		return 0, err
	}
	hooks, groups, err := sessionStart(cfg)
	if err != nil {
		return 0, err
	}
	command := HookCommand(woolly)
	res := Added
	if gi, hi := findHook(groups); gi >= 0 {
		h := groups[gi].(map[string]any)["hooks"].([]any)[hi].(map[string]any)
		if h["command"] == command {
			return Unchanged, nil
		}
		h["command"] = command
		res = Updated
	} else {
		groups = append(groups, map[string]any{
			"matcher": hookMatcher,
			"hooks": []any{map[string]any{
				"type": "command", "command": command, "timeout": 10,
			}},
		})
	}
	if dryRun {
		return res, nil
	}
	hooks["SessionStart"] = groups
	cfg["hooks"] = hooks
	return res, writeConfig(path, cfg, orig)
}

// RemoveStartupHook removes Woolly's hook and any lists it leaves empty.
func RemoveStartupHook(path string, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(path)
	if err != nil {
		return 0, err
	}
	if orig == nil {
		return Absent, nil
	}
	hooks, groups, err := sessionStart(cfg)
	if err != nil {
		return 0, err
	}
	if gi, _ := findHook(groups); gi < 0 {
		return Absent, nil
	}
	if dryRun {
		return Removed, nil
	}
	var kept []any
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			kept = append(kept, g)
			continue
		}
		list, _ := gm["hooks"].([]any)
		var rest []any
		for _, h := range list {
			if !isOurs(h) {
				rest = append(rest, h)
			}
		}
		if len(rest) == 0 && len(list) > 0 {
			continue
		}
		if len(rest) != len(list) {
			gm["hooks"] = rest
		}
		kept = append(kept, gm)
	}
	if len(kept) == 0 {
		delete(hooks, "SessionStart")
	} else {
		hooks["SessionStart"] = kept
	}
	if len(hooks) == 0 {
		delete(cfg, "hooks")
	} else {
		cfg["hooks"] = hooks
	}
	return Removed, writeConfig(path, cfg, orig)
}

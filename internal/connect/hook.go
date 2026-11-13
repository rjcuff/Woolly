package connect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookApp is the name Woolly's hooks give in the activity log.
const HookApp = "claude-code"

// hookSpec is one Claude Code hook Woolly adds. args ends the command, which
// is how Woolly finds its own hooks again.
type hookSpec struct {
	event, matcher, args string
	timeout              int
}

var hookSpecs = []hookSpec{
	// Load memory when a session starts, after /clear and after compaction.
	// Resumed sessions already have it.
	{event: "SessionStart", matcher: "startup|clear|compact", args: "context --app " + HookApp, timeout: 10},
	// Save where the session left off before it compacts and when it ends.
	{event: "PreCompact", args: "handoff --app " + HookApp, timeout: 10},
	{event: "SessionEnd", args: "handoff --app " + HookApp, timeout: 5},
}

// ClaudeSettings is Claude Code's user settings file.
func ClaudeSettings(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func quote(woolly string) string {
	if strings.ContainsAny(woolly, " \t'\"") {
		return `"` + strings.ReplaceAll(woolly, `"`, `\"`) + `"`
	}
	return woolly
}

// HookCommand is the command the startup hook runs.
func HookCommand(woolly string) string { return quote(woolly) + " " + hookSpecs[0].args }

func (s hookSpec) ours(h any) bool {
	m, ok := h.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.HasSuffix(strings.TrimSpace(cmd), s.args) && strings.Contains(strings.ToLower(cmd), "woolly")
}

// hooksOf returns settings.hooks and the groups for one event, checking shapes.
func hooksOf(cfg map[string]any, event string) (map[string]any, []any, error) {
	hooks := map[string]any{}
	if raw, ok := cfg["hooks"]; ok && raw != nil {
		h, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf(`"hooks" should be an object: %w`, ErrUnreadable)
		}
		hooks = h
	}
	var groups []any
	if raw, ok := hooks[event]; ok && raw != nil {
		g, ok := raw.([]any)
		if !ok {
			return nil, nil, fmt.Errorf(`"hooks.%s" should be a list: %w`, event, ErrUnreadable)
		}
		groups = g
	}
	return hooks, groups, nil
}

func (s hookSpec) find(groups []any) map[string]any {
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		list, _ := gm["hooks"].([]any)
		for _, h := range list {
			if s.ours(h) {
				return h.(map[string]any)
			}
		}
	}
	return nil
}

// HasStartupHook reports whether Claude Code's settings load Woolly at startup.
func HasStartupHook(path string) bool {
	cfg, _, err := readConfig(path)
	if err != nil {
		return false
	}
	_, groups, err := hooksOf(cfg, hookSpecs[0].event)
	return err == nil && hookSpecs[0].find(groups) != nil
}

// AddStartupHook adds Woolly's Claude Code hooks: load memory when a session
// starts, and save where it left off before compaction and when it ends.
func AddStartupHook(path, woolly string, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(path)
	if err != nil {
		return 0, err
	}
	res := Unchanged
	for _, s := range hookSpecs {
		hooks, groups, err := hooksOf(cfg, s.event)
		if err != nil {
			return 0, err
		}
		command := quote(woolly) + " " + s.args
		if h := s.find(groups); h != nil {
			if h["command"] != command {
				h["command"] = command
				if res == Unchanged {
					res = Updated
				}
			}
			continue
		}
		group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": s.timeout}}}
		if s.matcher != "" {
			group["matcher"] = s.matcher
		}
		hooks[s.event] = append(groups, group)
		cfg["hooks"] = hooks
		res = Added
	}
	if dryRun || res == Unchanged {
		return res, nil
	}
	return res, writeConfig(path, cfg, orig)
}

// RemoveStartupHook removes Woolly's hooks and any lists they leave empty.
func RemoveStartupHook(path string, dryRun bool) (Result, error) {
	cfg, orig, err := readConfig(path)
	if err != nil {
		return 0, err
	}
	if orig == nil {
		return Absent, nil
	}
	found := false
	for _, s := range hookSpecs {
		hooks, groups, err := hooksOf(cfg, s.event)
		if err != nil {
			return 0, err
		}
		if s.find(groups) == nil {
			continue
		}
		found = true
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
				if !s.ours(h) {
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
			delete(hooks, s.event)
		} else {
			hooks[s.event] = kept
		}
		if len(hooks) == 0 {
			delete(cfg, "hooks")
		} else {
			cfg["hooks"] = hooks
		}
	}
	if !found {
		return Absent, nil
	}
	if dryRun {
		return Removed, nil
	}
	return Removed, writeConfig(path, cfg, orig)
}

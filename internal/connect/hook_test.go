package connect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupHookAddAndRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	orig := `{
  "model": "opus",
  "cleanupPeriodDays": 30,
  "hooks": {
    "SessionStart": [{"matcher": "startup", "hooks": [{"type": "command", "command": "echo hi"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]
  }
}`
	os.WriteFile(path, []byte(orig), 0o600)

	if res, err := AddStartupHook(path, "/usr/local/bin/woolly", true); err != nil || res != Added {
		t.Fatalf("dry run: %v %v", res, err)
	}
	if data, _ := os.ReadFile(path); string(data) != orig {
		t.Fatal("dry run changed the file")
	}
	if res, err := AddStartupHook(path, "/usr/local/bin/woolly", false); err != nil || res != Added {
		t.Fatalf("add: %v %v", res, err)
	}
	if !HasStartupHook(path) {
		t.Fatal("hook not found after adding")
	}
	cfg := readJSON(t, path)
	if cfg["model"] != "opus" || cfg["cleanupPeriodDays"] != float64(30) {
		t.Fatalf("other settings lost: %v", cfg)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"echo hi", "say done", `"/usr/local/bin/woolly context --app claude-code"`, `"startup|clear|compact"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s in\n%s", want, data)
		}
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 && os.PathSeparator == '/' {
		t.Fatalf("mode changed to %v", st.Mode().Perm())
	}
	if res, _ := AddStartupHook(path, "/usr/local/bin/woolly", false); res != Unchanged {
		t.Fatalf("second add: %v", res)
	}
	if res, _ := AddStartupHook(path, "/opt/homebrew/bin/woolly", false); res != Updated {
		t.Fatalf("moved binary: %v", res)
	}
	if res, err := RemoveStartupHook(path, false); err != nil || res != Removed {
		t.Fatalf("remove: %v %v", res, err)
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "woolly") || !strings.Contains(string(data), "echo hi") || !strings.Contains(string(data), "say done") {
		t.Fatalf("remove touched the wrong hooks:\n%s", data)
	}
	if res, _ := RemoveStartupHook(path, false); res != Absent {
		t.Fatalf("second remove: %v", res)
	}
}

func TestStartupHookNewFileAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	if res, _ := RemoveStartupHook(path, false); res != Absent {
		t.Fatal("remove from a missing file should do nothing")
	}
	if _, err := AddStartupHook(path, `/Users/a b/woolly`, false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `\"/Users/a b/woolly\" context --app claude-code`) {
		t.Fatalf("path with a space should be quoted:\n%s", data)
	}
	RemoveStartupHook(path, false)
	if cfg := readJSON(t, path); len(cfg) != 0 {
		t.Fatalf("empty hooks should be tidied away: %v", cfg)
	}
}

func TestStartupHookRefusesOddShapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"hooks": {"SessionStart": "nope"}}`), 0o600)
	if _, err := AddStartupHook(path, "/bin/woolly", false); err == nil {
		t.Fatal("expected an error for a malformed hooks list")
	}
}

func TestHandoffHooksAreAddedAndRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := AddStartupHook(path, "/usr/local/bin/woolly", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`"PreCompact"`, `"SessionEnd"`, `woolly handoff --app claude-code`, `woolly context --app claude-code`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("missing %s in\n%s", want, data)
		}
	}
	if res, _ := AddStartupHook(path, "/usr/local/bin/woolly", false); res != Unchanged {
		t.Fatalf("second add: %v", res)
	}
	RemoveStartupHook(path, false)
	if cfg := readJSON(t, path); len(cfg) != 0 {
		t.Fatalf("all hooks should be gone: %v", cfg)
	}
}

func TestOlderInstallGetsTheNewHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"hooks":{"SessionStart":[{"matcher":"startup|clear|compact","hooks":[{"type":"command","command":"/usr/local/bin/woolly context --app claude-code","timeout":10}]}]}}`), 0o600)
	if res, _ := AddStartupHook(path, "/usr/local/bin/woolly", false); res != Added {
		t.Fatalf("got %v", res)
	}
	if data, _ := os.ReadFile(path); strings.Count(string(data), "context --app") != 1 || !strings.Contains(string(data), "PreCompact") {
		t.Fatalf("%s", data)
	}
}

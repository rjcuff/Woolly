package service

import (
	"encoding/xml"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func spec(home string) Spec {
	return Spec{
		Exe:  "/Users/a b/bin/woolly",
		Args: []string{"serve", "--proxy", "--log", "/Users/a b/.woolly/logs/woolly.log"},
		Log:  "/Users/a b/.woolly/logs/woolly.log",
		Env:  map[string]string{"WOOLLY_HOME": "/data/100% <mine>"},
		Home: home,
	}
}

func TestPlistIsValidXML(t *testing.T) {
	p := Plist(spec("/Users/a"))
	dec := xml.NewDecoder(strings.NewReader(p))
	dec.Strict = false
	for {
		if _, err := dec.Token(); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("plist is not valid XML: %v\n%s", err, p)
		}
	}
	for _, want := range []string{
		"<string>" + Label + "</string>",
		"<string>/Users/a b/bin/woolly</string>",
		"<string>--proxy</string>",
		"<key>WOOLLY_HOME</key>",
		"/data/100% &lt;mine&gt;",
		"<key>RunAtLoad</key>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist missing %q", want)
		}
	}
}

func TestUnitQuoting(t *testing.T) {
	u := Unit(spec("/home/a"))
	for _, want := range []string{
		`ExecStart="/Users/a b/bin/woolly" serve --proxy --log "/Users/a b/.woolly/logs/woolly.log"`,
		`Environment="WOOLLY_HOME=/data/100%% <mine>"`,
		"WantedBy=default.target",
		"Restart=on-failure",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("unit missing %q\n%s", want, u)
		}
	}
}

func TestSystemdQuote(t *testing.T) {
	tests := map[string]string{
		"serve":    "serve",
		"a b":      `"a b"`,
		`say "hi"`: `"say \"hi\""`,
		"$HOME":    `"$$HOME"`,
		`C:\x`:     `"C:\\x"`,
		"50%":      "50%%",
	}
	for in, want := range tests {
		if got := systemdQuote(in); got != want {
			t.Errorf("systemdQuote(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestWindowsCommand(t *testing.T) {
	s := Spec{Exe: `C:\Program Files\Woolly\woolly.exe`, Args: []string{"serve", "--proxy", "--log", `C:\Users\a\.woolly\logs\woolly.log`}}
	want := `conhost.exe --headless "C:\Program Files\Woolly\woolly.exe" serve --proxy --log C:\Users\a\.woolly\logs\woolly.log`
	if got := WindowsCommand(s); got != want {
		t.Fatalf("WindowsCommand =\n %s\nwant\n %s", got, want)
	}
}

type fakeRunner struct {
	calls []string
	fail  map[string]bool // command prefix -> fail
}

func (f *fakeRunner) run(name string, args ...string) ([]byte, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, line)
	for prefix := range f.fail {
		if strings.HasPrefix(line, prefix) {
			return []byte("boom"), errors.New("exit status 1")
		}
	}
	return nil, nil
}

func (f *fakeRunner) called(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func TestInstallLaunchd(t *testing.T) {
	home := t.TempDir()
	fr := &fakeRunner{fail: map[string]bool{"launchctl bootout": true}}
	m := &Manager{GOOS: "darwin", Run: fr.run, UID: 501}
	s := spec(home)
	s.Log = home + "/.woolly/logs/woolly.log"
	if err := m.Install(s); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !fr.called("launchctl bootstrap gui/501 " + PlistPath(home)) {
		t.Fatalf("bootstrap not called: %v", fr.calls)
	}
	if !m.Installed(home) {
		t.Fatal("Installed = false after Install")
	}
	if err := m.Uninstall(home); err != nil {
		t.Fatal(err)
	}
	if m.Installed(home) {
		t.Fatal("Installed = true after Uninstall")
	}
}

func TestInstallSystemd(t *testing.T) {
	home := t.TempDir()
	fr := &fakeRunner{}
	m := &Manager{GOOS: "linux", Run: fr.run}
	s := spec(home)
	s.Log = home + "/.woolly/logs/woolly.log"
	if err := m.Install(s); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !fr.called("systemctl --user enable --now woolly.service") {
		t.Fatalf("enable not called: %v", fr.calls)
	}
	if _, err := os.Stat(UnitPath(home)); err != nil {
		t.Fatal(err)
	}
	if err := m.Uninstall(home); err != nil {
		t.Fatal(err)
	}
	if !fr.called("systemctl --user disable --now woolly.service") {
		t.Fatalf("disable not called: %v", fr.calls)
	}
}

func TestInstallSystemdMissing(t *testing.T) {
	home := t.TempDir()
	fr := &fakeRunner{fail: map[string]bool{"systemctl --user --version": true}}
	m := &Manager{GOOS: "linux", Run: fr.run}
	s := spec(home)
	s.Log = home + "/logs/woolly.log"
	if err := m.Install(s); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v; want ErrUnsupported", err)
	}
}

func TestInstallWindows(t *testing.T) {
	home := t.TempDir()
	fr := &fakeRunner{}
	m := &Manager{GOOS: "windows", Run: fr.run}
	s := spec(home)
	s.Log = home + "/logs/woolly.log"
	if err := m.Install(s); err != nil {
		t.Fatal(err)
	}
	if !fr.called(`reg add ` + winRunKey + ` /v Woolly /t REG_SZ /d conhost.exe --headless`) {
		t.Fatalf("reg add not called: %v", fr.calls)
	}
	if m.StartsItself() {
		t.Fatal("Windows Install does not start the process")
	}
	if err := m.Uninstall(home); err != nil {
		t.Fatal(err)
	}
	if !fr.called("reg delete " + winRunKey + " /v Woolly /f") {
		t.Fatalf("reg delete not called: %v", fr.calls)
	}
}

func TestUnsupported(t *testing.T) {
	m := &Manager{GOOS: "plan9", Run: (&fakeRunner{}).run}
	if err := m.Install(Spec{Log: t.TempDir() + "/w.log"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
}

func TestLaunchdRetriesBootstrap(t *testing.T) {
	home := t.TempDir()
	tries := 0
	m := &Manager{GOOS: "darwin", UID: 501, RetryDelay: time.Millisecond, Run: func(name string, args ...string) ([]byte, error) {
		if name == "launchctl" && args[0] == "bootstrap" {
			tries++
			if tries < 3 {
				return []byte("Bootstrap failed: 5: Input/output error"), errors.New("exit status 5")
			}
		}
		return nil, nil
	}}
	s := spec(home)
	s.Log = home + "/logs/woolly.log"
	if err := m.Install(s); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if tries != 3 {
		t.Fatalf("bootstrap tries = %d; want 3", tries)
	}
}

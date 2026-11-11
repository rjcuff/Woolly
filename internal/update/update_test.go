package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.6.0", "0.5.0", true},
		{"0.5.0", "0.5.0", false},
		{"0.5.0", "0.6.0", false},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"0.6.0", "0.1.0-dev", true},
		{"0.6.0", "0.6.0+dirty", false},
		{"0.6.0", "devel", true},
		{"nonsense", "0.1.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestManagedBy(t *testing.T) {
	cases := map[string]string{
		"/opt/homebrew/bin/woolly":                     "homebrew",
		"/usr/local/Cellar/woolly/0.5.0/bin/woolly":    "homebrew",
		"/home/a/.linuxbrew/bin/woolly":                "",
		"/home/linuxbrew/.linuxbrew/bin/woolly":        "homebrew",
		"/Users/a/go/bin/woolly":                       "go",
		"/usr/local/bin/woolly":                        "",
		"/Users/a/AppData/Local/Programs/woolly/w.exe": "",
	}
	for exe, want := range cases {
		if got := ManagedBy(exe); got != want {
			t.Errorf("ManagedBy(%q) = %q, want %q", exe, got, want)
		}
	}
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// fakeGitHub serves a release with one archive and its checksum.
func fakeGitHub(t *testing.T, archive []byte, sum string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	})
	mux.HandleFunc("/"+Repo+"/releases/download/v9.9.9/woolly_darwin_arm64.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	})
	mux.HandleFunc("/"+Repo+"/releases/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  woolly_darwin_arm64.tar.gz\n%s  woolly_linux_amd64.tar.gz\n", sum, "00")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{APIBase: srv.URL, DownloadBase: srv.URL, HTTP: srv.Client(), GOOS: "darwin", GOARCH: "arm64"}
}

func TestDownloadAndReplace(t *testing.T) {
	archive := tarGz(t, "woolly", []byte("new binary"))
	sum := sha256.Sum256(archive)
	c := fakeGitHub(t, archive, hex.EncodeToString(sum[:]))

	v, err := c.Latest(context.Background())
	if err != nil || v != "9.9.9" {
		t.Fatalf("latest: %q %v", v, err)
	}
	bin, err := c.Download(context.Background(), v)
	if err != nil || string(bin) != "new binary" {
		t.Fatalf("download: %q %v", bin, err)
	}

	exe := filepath.Join(t.TempDir(), "woolly")
	os.WriteFile(exe, []byte("old binary"), 0o755)
	if err := Replace(exe, bin); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	info, _ := os.Stat(exe)
	if string(got) != "new binary" || info.Mode().Perm() != 0o755 {
		t.Fatalf("replaced: %q %v", got, info.Mode())
	}
}

func TestRejectsBadChecksum(t *testing.T) {
	archive := tarGz(t, "woolly", []byte("tampered"))
	c := fakeGitHub(t, archive, "deadbeef")
	if _, err := c.Download(context.Background(), "9.9.9"); err == nil {
		t.Fatal("expected a checksum error")
	}
}

// Package update installs the latest release from GitHub after checking it
// against the published checksums. It's the only code that goes online.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is where releases are published.
const Repo = "rjcuff/woolly"

// Client checks for and downloads releases. The URLs can be changed for tests.
type Client struct {
	APIBase      string // https://api.github.com
	DownloadBase string // https://github.com
	HTTP         *http.Client
	GOOS, GOARCH string
}

// New returns a client for the real GitHub.
func New() *Client {
	return &Client{
		APIBase:      "https://api.github.com",
		DownloadBase: "https://github.com",
		HTTP:         &http.Client{Timeout: 2 * time.Minute},
		GOOS:         runtime.GOOS,
		GOARCH:       runtime.GOARCH,
	}
}

// Latest returns the newest released version, without a leading "v".
func (c *Client) Latest(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIBase+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("can't reach GitHub to check for updates: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checking for updates: GitHub returned %s", res.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&rel); err != nil {
		return "", fmt.Errorf("checking for updates: %w", err)
	}
	if rel.TagName == "" {
		return "", errors.New("checking for updates: no release found")
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// Newer reports whether version a is newer than b. Both look like 1.2.3;
// anything after a "-" or "+" is ignored, and an unparseable b (like a
// development build) counts as older.
func Newer(a, b string) bool {
	pa, okA := parse(a)
	pb, okB := parse(b)
	if !okA {
		return false
	}
	if !okB {
		return true
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// ManagedBy says whether a package manager owns this binary, in which case
// updating should go through it. It returns "" for the install script.
func ManagedBy(exe string) string {
	p := filepath.ToSlash(exe)
	switch {
	case strings.Contains(p, "/Cellar/") || strings.Contains(p, "/homebrew/") || strings.Contains(p, "/linuxbrew/"):
		return "homebrew"
	case strings.Contains(p, "/go/bin/") || (os.Getenv("GOBIN") != "" && strings.HasPrefix(p, filepath.ToSlash(os.Getenv("GOBIN")))):
		return "go"
	}
	return ""
}

// Download fetches the release archive for this computer, verifies it against
// the release's checksums.txt, and returns the woolly binary inside it.
func (c *Client) Download(ctx context.Context, version string) ([]byte, error) {
	ext := ".tar.gz"
	if c.GOOS == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("woolly_%s_%s%s", c.GOOS, c.GOARCH, ext)
	base := fmt.Sprintf("%s/%s/releases/download/v%s/", c.DownloadBase, Repo, version)

	archive, err := c.get(ctx, base+name)
	if err != nil {
		return nil, err
	}
	sums, err := c.get(ctx, base+"checksums.txt")
	if err != nil {
		return nil, err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	got := sha256.Sum256(archive)
	if want == "" || !strings.EqualFold(want, hex.EncodeToString(got[:])) {
		return nil, errors.New("the download doesn't match the published checksum. Nothing was changed")
	}

	bin := "woolly"
	if c.GOOS == "windows" {
		return fromZip(archive, "woolly.exe")
	}
	return fromTarGz(archive, bin)
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 200<<20))
}

func fromTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == name && h.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, 200<<20))
		}
	}
	return nil, fmt.Errorf("no %s in the download", name)
}

func fromZip(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 200<<20))
		}
	}
	return nil, fmt.Errorf("no %s in the download", name)
}

// Replace swaps the binary at exe for the new one. The new file is written
// next to it and renamed into place, so a failure never leaves a broken
// binary. On Windows the running file is moved aside first.
func Replace(exe string, bin []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".woolly-update-*")
	if err != nil {
		return fmt.Errorf("can't write to %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
	}
	return os.Rename(tmp.Name(), exe)
}

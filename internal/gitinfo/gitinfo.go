// Package gitinfo asks git which commit a repo is at and what changed since.
package gitinfo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

// Head returns the repo's current commit, or "" if dir isn't a git repo.
func Head(dir string) string {
	h, err := git(dir, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return ""
	}
	return h
}

var pathLike = regexp.MustCompile("`?([A-Za-z0-9_.\\-]+(?:/[A-Za-z0-9_.\\-]+)*\\.[A-Za-z0-9]{1,8}|[A-Za-z0-9_\\-]+(?:/[A-Za-z0-9_.\\-]+)+)`?")

// Paths returns the files and folders text mentions that exist in the repo.
func Paths(dir, text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range pathLike.FindAllStringSubmatch(text, 20) {
		p := strings.Trim(m[1], "./")
		if p == "" || seen[p] || strings.Contains(p, "..") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// Changes counts the commits since commit that touched any of paths.
func Changes(dir, commit string, paths []string) int {
	if commit == "" || len(paths) == 0 {
		return 0
	}
	out, err := git(dir, append([]string{"rev-list", "--count", commit + "..HEAD", "--"}, paths...)...)
	if err != nil {
		return 0
	}
	n := 0
	for _, c := range out {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

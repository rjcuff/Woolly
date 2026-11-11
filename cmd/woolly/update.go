package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rjcuff/woolly/internal/update"
)

// cmdUpdate checks GitHub for a newer release and installs it. It's the only
// command that contacts the internet, and only when you run it.
func cmdUpdate(args []string) error {
	fs := newFlags("update")
	check := fs.Bool("check", false, "only check whether a newer version exists")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	c := update.New()
	latest, err := c.Latest(ctx)
	if err != nil {
		return err
	}
	if !update.Newer(latest, version) {
		fmt.Printf("You're on the latest version (%s).\n", version)
		return nil
	}
	fmt.Printf("Woolly %s is available. You have %s.\n", latest, version)
	fmt.Println("What's new: https://github.com/" + update.Repo + "/releases/tag/v" + latest)
	if *check {
		fmt.Println("Update with: woolly update")
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	switch update.ManagedBy(exe) {
	case "homebrew":
		fmt.Println("\nWoolly was installed with Homebrew. Update with:\n  brew upgrade woolly")
		return nil
	case "go":
		fmt.Println("\nWoolly was installed with Go. Update with:\n  go install github.com/" + update.Repo + "/cmd/woolly@latest")
		return nil
	}

	fmt.Println("Downloading...")
	bin, err := c.Download(ctx, latest)
	if err != nil {
		return err
	}
	if err := update.Replace(exe, bin); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("no permission to replace %s. Run: sudo woolly update", exe)
		}
		return err
	}
	fmt.Printf("Updated to %s. Checksum verified.\n", latest)
	fmt.Println("If Woolly runs in the background, restart it with: woolly service install")
	return nil
}

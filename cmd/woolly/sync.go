package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/peer"
)

const syncUsage = `Keep your memories in step between your own devices.

  woolly sync init                 set up sync on this device and print a join command
  woolly sync join <addr> <code>   link this device to another one
  woolly sync                      sync with your other devices now
  woolly sync status               list linked devices
  woolly sync serve                listen for your other devices (woolly serve does this too)
  woolly sync leave                stop syncing on this device
`

func cmdSync(args []string) error {
	if len(args) == 0 {
		return syncNow()
	}
	switch args[0] {
	case "init":
		return syncInit()
	case "join":
		if len(args) != 3 {
			return errors.New("usage: woolly sync join <address> <code>")
		}
		return syncJoin(args[1], args[2])
	case "status":
		return syncStatus()
	case "serve":
		return syncServe()
	case "leave":
		return syncLeave()
	case "help", "-h", "--help":
		fmt.Print(syncUsage)
		return nil
	}
	return fmt.Errorf("unknown sync command %q\n\n%s", args[0], syncUsage)
}

func syncAddr() string {
	return envOr("WOOLLY_SYNC_ADDR", "0.0.0.0:"+peer.DefaultPort)
}

func syncInit() error {
	a, err := open()
	if err != nil {
		return err
	}
	cfg, err := peer.Load(a.home)
	if err != nil {
		return err
	}
	if cfg == nil {
		if cfg, err = peer.NewConfig(); err != nil {
			return err
		}
		if err := cfg.Save(a.home); err != nil {
			return err
		}
	}
	ip := "<this-device's-address>"
	if addrs := peer.LocalAddrs(); len(addrs) > 0 {
		ip = addrs[0]
	}
	fmt.Printf(`Sync is set up on this device.

On your other device, run:

  woolly sync join %s %s

Keep the code private. Anyone with it can read and change your memories.
This device needs to be running Woolly (woolly serve or woolly service install)
for the other one to reach it.
`, ip, cfg.JoinCode())
	return nil
}

func syncJoin(addr, code string) error {
	a, err := open()
	if err != nil {
		return err
	}
	cfg, err := peer.Load(a.home)
	if err != nil {
		return err
	}
	if cfg == nil {
		if cfg, err = peer.NewConfig(); err != nil {
			return err
		}
	}
	if !peer.ValidCode(code) {
		return errors.New("that code doesn't look right. Copy it again from `woolly sync init`")
	}
	if cfg.Key != code {
		if len(cfg.Peers) > 0 {
			return errors.New("this device already syncs with a different group. Run `woolly sync leave` first")
		}
		cfg.Key = code
	}
	addr = peer.NormalizeAddr(addr)
	cfg.AddPeer(addr)
	if err := cfg.Save(a.home); err != nil {
		return err
	}
	fmt.Println("Linked with", addr)
	return syncNow()
}

func syncNow() error {
	a, err := open()
	if err != nil {
		return err
	}
	cfg, err := peer.Load(a.home)
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.Peers) == 0 {
		fmt.Print("Sync isn't set up yet.\n\n" + syncUsage)
		return nil
	}
	failed := syncAll(a, cfg, true)
	if failed > 0 {
		return fmt.Errorf("%d of %d devices didn't sync", failed, len(cfg.Peers))
	}
	return nil
}

// syncAll syncs with every peer, records the result and returns how many failed.
func syncAll(a *app, cfg *peer.Config, verbose bool) int {
	client := &peer.Client{}
	failed := 0
	for i := range cfg.Peers {
		p := &cfg.Peers[i]
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		res, err := client.Sync(ctx, a.store, cfg, p.Addr)
		cancel()
		if err != nil {
			failed++
			p.LastErr = err.Error()
			if verbose {
				fmt.Printf("  ! %s: %v\n", p.Addr, err)
			}
			continue
		}
		p.LastErr, p.LastSync = "", time.Now().UTC()
		if res.Name != "" {
			p.Name = res.Name
		}
		if verbose {
			fmt.Printf("  ✓ %s: got %d, sent %d", label(*p), res.Received, res.Sent)
			if res.Deleted > 0 {
				fmt.Printf(", deleted %d", res.Deleted)
			}
			fmt.Println()
		}
		if res.Received+res.Sent+res.Deleted > 0 {
			a.log.Add(activity.Entry{App: label(*p), Action: "sync", Count: res.Received + res.Sent + res.Deleted})
		}
	}
	_ = cfg.Save(a.home)
	return failed
}

func label(p peer.Peer) string {
	if p.Name != "" {
		return p.Name + " (" + p.Addr + ")"
	}
	return p.Addr
}

func syncStatus() error {
	a, err := open()
	if err != nil {
		return err
	}
	cfg, err := peer.Load(a.home)
	if err != nil {
		return err
	}
	if cfg == nil {
		fmt.Println("Sync isn't set up. Run: woolly sync init")
		return nil
	}
	fmt.Println("This device:", cfg.Name)
	fmt.Println("Listens on:", syncAddr())
	if len(cfg.Peers) == 0 {
		fmt.Println("No linked devices yet. On another device, run the join command from `woolly sync init`.")
		return nil
	}
	for _, p := range cfg.Peers {
		when := "never"
		if !p.LastSync.IsZero() {
			when = p.LastSync.Local().Format("Jan 02 15:04")
		}
		fmt.Printf("  %s  last sync: %s\n", label(p), when)
		if p.LastErr != "" {
			fmt.Printf("    last error: %s\n", p.LastErr)
		}
	}
	return nil
}

func syncLeave() error {
	a, err := open()
	if err != nil {
		return err
	}
	if err := os.Remove(peer.ConfigPath(a.home)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fmt.Println("Sync is off on this device. Your memories are still here.")
	return nil
}

func syncServe() error {
	a, err := open()
	if err != nil {
		return err
	}
	stop, err := startSync(a, true)
	if err != nil {
		return err
	}
	if stop == nil {
		return errors.New("sync isn't set up. Run: woolly sync init")
	}
	select {}
}

// ensureSync starts the sync listener once, if sync is set up.
func (a *app) ensureSync() error {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.syncStop != nil {
		return nil
	}
	stop, err := startSync(a, false)
	a.syncStop = stop
	return err
}

// startSync listens for other devices and syncs every 5 minutes. It returns
// nil when sync isn't set up.
func startSync(a *app, verbose bool) (func(), error) {
	cfg, err := peer.Load(a.home)
	if err != nil || cfg == nil {
		return nil, err
	}
	h, err := peer.Handler(a.store, cfg)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", syncAddr())
	if err != nil {
		return nil, fmt.Errorf("can't listen for other devices on %s: %w", syncAddr(), err)
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	if verbose {
		fmt.Fprintf(os.Stderr, "Listening for your other devices on %s\n", syncAddr())
	}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			if c, err := peer.Load(a.home); err == nil && c != nil && len(c.Peers) > 0 {
				syncAll(a, c, false)
			}
			select {
			case <-t.C:
			case <-done:
				return
			}
		}
	}()
	return func() { close(done); srv.Close() }, nil
}

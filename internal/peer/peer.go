// Package peer syncs memories between your own devices over your network.
// Devices share one key from the join code, and every message is encrypted
// and authenticated with it (AES-256-GCM).
package peer

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rjcuff/woolly/internal/memory"
)

// DefaultPort is where Woolly listens for your other devices.
const DefaultPort = "11437"

// maxBody caps a sync message.
const maxBody = 64 << 20

// maxSkew limits clock difference, so old requests can't be replayed.
const maxSkew = 5 * time.Minute

// Config is ~/.woolly/sync.json.
type Config struct {
	Key   string `json:"key"` // base64url, 32 bytes
	Name  string `json:"name,omitempty"`
	Peers []Peer `json:"peers,omitempty"`
}

// Peer is another device.
type Peer struct {
	Addr     string    `json:"addr"`
	Name     string    `json:"name,omitempty"`
	LastSync time.Time `json:"last_sync,omitempty"`
	LastErr  string    `json:"last_error,omitempty"`
}

// ConfigPath is where the sync settings live.
func ConfigPath(home string) string { return filepath.Join(home, "sync.json") }

// Load reads the config. A missing file means sync isn't set up.
func Load(home string) (*Config, error) {
	data, err := os.ReadFile(ConfigPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("reading %s: %w", ConfigPath(home), err)
	}
	if _, err := c.key(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config, readable only by you.
func (c *Config) Save(home string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	path := ConfigPath(home)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Remove turns sync off on this device.
func Remove(home string) error {
	if err := os.Remove(ConfigPath(home)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// NewConfig creates a config with a new random key.
func NewConfig() (*Config, error) {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	host, _ := os.Hostname()
	return &Config{Key: base64.RawURLEncoding.EncodeToString(k), Name: host}, nil
}

func (c *Config) key() ([]byte, error) {
	k, err := base64.RawURLEncoding.DecodeString(c.Key)
	if err != nil || len(k) != 32 {
		return nil, errors.New("the sync key is damaged. Run `woolly sync init` again")
	}
	return k, nil
}

// ValidCode reports whether a join code is well formed.
func ValidCode(code string) bool {
	k, err := base64.RawURLEncoding.DecodeString(code)
	return err == nil && len(k) == 32
}

// AddPeer remembers a device, once.
func (c *Config) AddPeer(addr string) {
	for _, p := range c.Peers {
		if p.Addr == addr {
			return
		}
	}
	c.Peers = append(c.Peers, Peer{Addr: addr})
}

// JoinCode is the code other devices join with.
func (c *Config) JoinCode() string { return c.Key }

// NormalizeAddr adds the default port if it's missing.
func NormalizeAddr(addr string) string {
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	addr = strings.TrimSuffix(addr, "/")
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, DefaultPort)
	}
	return addr
}

// --- messages ---

type request struct {
	Op         string    `json:"op"` // manifest, get, put
	Time       time.Time `json:"time"`
	IDs        []string  `json:"ids,omitempty"`
	Files      []File    `json:"files,omitempty"`
	Tombstones []string  `json:"tombstones,omitempty"`
	From       string    `json:"from,omitempty"`
}

// File is one memory file in transit.
type File struct {
	Data  []byte `json:"data"`
	Trash bool   `json:"trash,omitempty"`
}

type reply struct {
	Time       time.Time     `json:"time"`
	Error      string        `json:"error,omitempty"`
	Items      []memory.Item `json:"items,omitempty"`
	Tombstones []string      `json:"tombstones,omitempty"`
	Files      []File        `json:"files,omitempty"`
	Applied    int           `json:"applied,omitempty"`
	Name       string        `json:"name,omitempty"`
}

// seal encrypts v. The label stops a reply being reused as a request.
func seal(key []byte, label string, v any) ([]byte, error) {
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, []byte(label)), nil
}

var errAuth = errors.New("couldn't verify the other device. Both devices need the same join code")

func open(key []byte, label string, data []byte, v any) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(data) < gcm.NonceSize() {
		return errAuth
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(label))
	if err != nil {
		return errAuth
	}
	return json.Unmarshal(plain, v)
}

func fresh(t time.Time) bool {
	d := time.Since(t)
	return d < maxSkew && d > -maxSkew
}

// --- server ---

// Handler answers other devices. Anything not sealed with the key gets a 403.
func Handler(store *memory.Store, cfg *Config) (http.Handler, error) {
	key, err := cfg.key()
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex // one sync at a time
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/woolly-sync" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		var req request
		if err := open(key, "request", body, &req); err != nil || !fresh(req.Time) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mu.Lock()
		rep := answer(store, cfg, &req)
		mu.Unlock()
		rep.Time = time.Now().UTC()
		out, err := seal(key, "reply", rep)
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(out)
	}), nil
}

func answer(store *memory.Store, cfg *Config, req *request) reply {
	switch req.Op {
	case "manifest":
		items, err := store.Manifest()
		if err != nil {
			return reply{Error: err.Error()}
		}
		return reply{Items: items, Tombstones: store.Tombstones(), Name: cfg.Name}
	case "get":
		var files []File
		for _, id := range req.IDs {
			data, trash, err := store.Raw(id)
			if err == nil {
				files = append(files, File{Data: data, Trash: trash})
			}
		}
		return reply{Files: files}
	case "put":
		n, err := applyAll(store, req.Files, req.Tombstones)
		if err != nil {
			return reply{Error: err.Error(), Applied: n}
		}
		return reply{Applied: n}
	}
	return reply{Error: "unknown request"}
}

func applyAll(store *memory.Store, files []File, tombstones []string) (int, error) {
	n := 0
	for _, id := range tombstones {
		if err := store.Delete(id); err != nil {
			return n, err
		}
	}
	for _, f := range files {
		ok, err := store.Apply(f.Data, f.Trash)
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// --- client ---

// Result says what one sync did.
type Result struct {
	Peer     string
	Name     string
	Received int
	Sent     int
	Deleted  int
}

// Client talks to other devices.
type Client struct {
	HTTP *http.Client
}

func (c *Client) call(ctx context.Context, key []byte, addr string, req request) (*reply, error) {
	req.Time = time.Now().UTC()
	body, err := seal(key, "request", req)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/woolly-sync", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 2 * time.Minute}
	}
	res, err := hc.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("can't reach %s. Is Woolly running there, on the same network? %w", addr, err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%s refused to sync: %w", addr, errAuth)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", addr, res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	var rep reply
	if err := open(key, "reply", data, &rep); err != nil {
		return nil, err
	}
	if !fresh(rep.Time) {
		return nil, errAuth
	}
	if rep.Error != "" {
		return &rep, fmt.Errorf("%s: %s", addr, rep.Error)
	}
	return &rep, nil
}

// Sync brings this device and the one at addr up to date with each other.
func (c *Client) Sync(ctx context.Context, store *memory.Store, cfg *Config, addr string) (Result, error) {
	res := Result{Peer: addr}
	key, err := cfg.key()
	if err != nil {
		return res, err
	}
	remote, err := c.call(ctx, key, addr, request{Op: "manifest", From: cfg.Name})
	if err != nil {
		return res, err
	}
	res.Name = remote.Name

	// Deleted for good on either side means deleted on both.
	theirDead := map[string]bool{}
	for _, id := range remote.Tombstones {
		theirDead[id] = true
	}
	ourDead := map[string]bool{}
	for _, id := range store.Tombstones() {
		ourDead[id] = true
	}
	for id := range theirDead {
		if !ourDead[id] {
			if err := store.Delete(id); err != nil {
				return res, err
			}
			res.Deleted++
		}
	}

	local, err := store.Manifest()
	if err != nil {
		return res, err
	}
	mine := map[string]memory.Item{}
	for _, it := range local {
		mine[it.ID] = it
	}
	theirs := map[string]memory.Item{}
	var want []string
	for _, it := range remote.Items {
		if !memory.ValidID(it.ID) || ourDead[it.ID] || theirDead[it.ID] {
			continue
		}
		theirs[it.ID] = it
		if m, ok := mine[it.ID]; !ok || it.Newer(m) {
			want = append(want, it.ID)
		}
	}
	var give []File
	for id, it := range mine {
		if t, ok := theirs[id]; ok && !it.Newer(t) {
			continue
		}
		data, trash, err := store.Raw(id)
		if err == nil {
			give = append(give, File{Data: data, Trash: trash})
		}
	}

	for len(want) > 0 {
		n := min(len(want), 500)
		got, err := c.call(ctx, key, addr, request{Op: "get", IDs: want[:n]})
		if err != nil {
			return res, err
		}
		applied, err := applyAll(store, got.Files, nil)
		res.Received += applied
		if err != nil {
			return res, err
		}
		want = want[n:]
	}

	var newDead []string
	for id := range ourDead {
		if !theirDead[id] {
			newDead = append(newDead, id)
		}
	}
	for first := true; first || len(give) > 0; first = false {
		n := min(len(give), 500)
		rep, err := c.call(ctx, key, addr, request{Op: "put", Files: give[:n], Tombstones: newDead})
		if err != nil {
			return res, err
		}
		res.Sent += rep.Applied
		give, newDead = give[n:], nil
	}
	return res, nil
}

// LocalAddrs lists this computer's private network addresses.
func LocalAddrs() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ip := ipnet.IP.To4(); ip != nil && ip.IsPrivate() {
				out = append(out, ip.String())
			}
		}
	}
	return out
}

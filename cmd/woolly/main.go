// Command woolly is a local-first memory for your AI tools.
//
// Ollama runs your models. Woolly remembers for them.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rjcuff/woolly/internal/importer"
	"github.com/rjcuff/woolly/internal/mcp"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/ollama"
	"github.com/rjcuff/woolly/internal/profile"
	"github.com/rjcuff/woolly/internal/proxy"
	"github.com/rjcuff/woolly/internal/recall"
	"github.com/rjcuff/woolly/internal/server"
)

var version = "0.1.0-dev"

// go install builds don't get the version stamped in, so read it from the
// module's build info instead.
func init() {
	if version != "0.1.0-dev" {
		return
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
}

const usage = `Woolly: local memory for your AI tools.

Usage:
  woolly remember <text> [--key k] [--tags a,b] [--source user|agent|import] [--allow-secret]
  woolly recall <query> [-n 5] [--all] [--json]
  woolly list [--all] [--json]
  woolly show <id>
  woolly forget <id>           move a memory to the trash
  woolly restore <id>          bring it back
  woolly trash [--empty]       list the trash, or delete it for good
  woolly import <chatgpt|claude> <export.zip> [--dry-run]
  woolly profile [--json]       summary of everything Woolly knows about you
  woolly conflicts
  woolly resolve <id>          keep this memory as the current fact for its key
  woolly ui                    open the review page in your browser
  woolly serve                 run the local HTTP API and review page (default 127.0.0.1:11435)
  woolly mcp                   run as an MCP server on stdio (for Claude, Cursor, ...)
  woolly proxy                 give memory to Ollama and OpenAI or Anthropic apps (127.0.0.1:11436)
  woolly doctor                check your setup
  woolly version

Memories are plain Markdown files in ~/.woolly/memories (set WOOLLY_HOME to move them).
If Ollama is running, Woolly uses it for semantic search; otherwise keyword search.

Connect Claude Code:   claude mcp add woolly -- woolly mcp
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "woolly:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "version", "--version", "-v":
		fmt.Println("woolly", version)
		return nil
	case "remember", "add":
		return cmdRemember(rest)
	case "recall", "search":
		return cmdRecall(rest)
	case "list", "ls":
		return cmdList(rest)
	case "show":
		return cmdShow(rest)
	case "forget", "rm":
		return cmdForget(rest)
	case "import":
		return cmdImport(rest)
	case "profile":
		return cmdProfile(rest)
	case "restore":
		return cmdRestore(rest)
	case "trash":
		return cmdTrash(rest)
	case "conflicts":
		return cmdConflicts(rest)
	case "resolve":
		return cmdResolve(rest)
	case "ui":
		return cmdUI(rest)
	case "serve":
		return cmdServe(rest)
	case "mcp":
		return cmdMCP(rest)
	case "proxy":
		return cmdProxy(rest)
	case "doctor":
		return cmdDoctor(rest)
	}
	return fmt.Errorf("unknown command %q (run 'woolly help')", cmd)
}

// home returns Woolly's data directory.
func home() (string, error) {
	if h := strings.TrimSpace(os.Getenv("WOOLLY_HOME")); h != "" {
		return h, nil
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".woolly"), nil
}

type app struct {
	home   string
	store  *memory.Store
	engine *recall.Engine
	ollama *ollama.Client
}

func open() (*app, error) {
	h, err := home()
	if err != nil {
		return nil, err
	}
	store, err := memory.Open(filepath.Join(h, "memories"))
	if err != nil {
		return nil, err
	}
	oc := ollama.FromEnv()
	var emb recall.Embedder = oc
	if os.Getenv("WOOLLY_NO_EMBED") != "" {
		emb = nil
	}
	engine := recall.New(store, emb, filepath.Join(h, "index", "embeddings.json"))
	return &app{home: h, store: store, engine: engine, ollama: oc}, nil
}

// parse parses flags that may appear before, between or after positional args.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("woolly "+name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func cmdRemember(args []string) error {
	fs := newFlags("remember")
	key := fs.String("key", "", "stable slot for facts that change, e.g. user.location")
	tags := fs.String("tags", "", "comma-separated tags")
	source := fs.String("source", "user", "who said it: user, agent or import")
	asJSON := fs.Bool("json", false, "print JSON")
	allowSecret := fs.Bool("allow-secret", false, "save even if it looks like a password or API key")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Join(pos, " "))
	if text == "" {
		return errors.New(`nothing to remember; try: woolly remember "I prefer tabs over spaces"`)
	}
	src, ok := memory.ParseSource(*source)
	if !ok {
		return fmt.Errorf("unknown --source %q", *source)
	}
	a, err := open()
	if err != nil {
		return err
	}
	res, err := a.store.Remember(memory.RememberInput{
		Content: text, Key: *key, Tags: []string{*tags}, Source: src, AllowSensitive: *allowSecret,
	})
	var sensitive *memory.SensitiveError
	if errors.As(err, &sensitive) {
		return fmt.Errorf("%v. If you're sure, add --allow-secret", err)
	}
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(res)
	}
	fmt.Println(mcp.DescribeRemember(res))
	return nil
}

func cmdRecall(args []string) error {
	fs := newFlags("recall")
	n := fs.Int("n", 5, "max results")
	all := fs.Bool("all", false, "include superseded and conflicting memories")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	query := strings.Join(pos, " ")
	a, err := open()
	if err != nil {
		return err
	}
	results, mode, err := a.engine.Recall(context.Background(), query, *n, *all)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(map[string]any{"mode": mode, "results": results})
	}
	if len(results) == 0 {
		fmt.Println("No matching memories.")
	}
	for _, r := range results {
		fmt.Printf("%.2f  %s\n", r.Score, mcp.Line(r.Memory))
	}
	if mode == recall.ModeKeyword && os.Getenv("WOOLLY_NO_EMBED") == "" {
		fmt.Fprintln(os.Stderr, "(keyword search; run 'woolly doctor' to enable semantic search)")
	}
	return nil
}

func cmdList(args []string) error {
	fs := newFlags("list")
	all := fs.Bool("all", false, "include superseded and conflicting memories")
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	mems, err := a.store.List(*all)
	if err != nil {
		return err
	}
	if *asJSON {
		return printJSON(mems)
	}
	if len(mems) == 0 {
		fmt.Println(`No memories yet. Try: woolly remember "I prefer dark mode"`)
	}
	for _, m := range mems {
		fmt.Println(mcp.Line(m))
	}
	return nil
}

func oneID(name string, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("usage: woolly %s <id>", name)
	}
	return args[0], nil
}

func cmdShow(args []string) error {
	id, err := oneID("show", args)
	if err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	m, err := a.store.Get(id)
	if err != nil {
		return err
	}
	return printJSON(m)
}

func cmdForget(args []string) error {
	id, err := oneID("forget", args)
	if err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	m, err := a.store.Forget(id)
	if err != nil {
		return err
	}
	fmt.Println("Moved to trash:", m.Content)
	fmt.Println("Undo with: woolly restore", m.ID)
	return nil
}

func cmdRestore(args []string) error {
	id, err := oneID("restore", args)
	if err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	m, err := a.store.Restore(id)
	if err != nil {
		return err
	}
	fmt.Println("Restored:", m.Content)
	return nil
}

func cmdTrash(args []string) error {
	fs := newFlags("trash")
	empty := fs.Bool("empty", false, "permanently delete everything in the trash")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	if *empty {
		n, err := a.store.EmptyTrash()
		if err != nil {
			return err
		}
		fmt.Printf("Permanently deleted %d memories.\n", n)
		return nil
	}
	mems, err := a.store.Trash()
	if err != nil {
		return err
	}
	if len(mems) == 0 {
		fmt.Println("The trash is empty.")
		return nil
	}
	for _, m := range mems {
		fmt.Println(mcp.Line(m))
	}
	fmt.Println("\nRestore one with: woolly restore <id>. Delete all for good with: woolly trash --empty")
	return nil
}

func cmdImport(args []string) error {
	fs := newFlags("import")
	dry := fs.Bool("dry-run", false, "show what would be imported without saving")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return errors.New("usage: woolly import <chatgpt|claude> <export.zip or conversations.json> [--dry-run]")
	}
	provider := strings.ToLower(pos[0])
	msgs, err := importer.Load(provider, pos[1])
	if err != nil {
		return err
	}
	found := importer.Extract(msgs)
	fmt.Printf("Read %d of your messages from %s and found %d facts.\n", len(msgs), provider, len(found))
	if len(found) == 0 {
		return nil
	}
	if *dry {
		for _, c := range found {
			key := ""
			if c.Key != "" {
				key = "  (" + c.Key + ")"
			}
			fmt.Printf("  %s%s\n", c.Content, key)
		}
		fmt.Println("\nNothing was saved. Run again without --dry-run to import.")
		return nil
	}
	a, err := open()
	if err != nil {
		return err
	}
	var added, dupes, conflicts, blocked int
	for _, c := range found {
		res, err := a.store.Remember(memory.RememberInput{
			Content: c.Content, Key: c.Key, Tags: append([]string{provider}, c.Tags...), Source: memory.SourceImport,
		})
		var sensitive *memory.SensitiveError
		var injected *memory.InjectionError
		if errors.As(err, &sensitive) || errors.As(err, &injected) {
			blocked++
			continue
		}
		if err != nil {
			return err
		}
		switch {
		case res.Duplicate:
			dupes++
		case res.ConflictsWith != nil:
			conflicts++
		default:
			added++
		}
	}
	fmt.Printf("Imported %d new memories (%d already known).\n", added, dupes)
	if blocked > 0 {
		fmt.Printf("Skipped %d that looked like passwords, keys or instructions to an AI.\n", blocked)
	}
	if conflicts > 0 {
		fmt.Printf("%d disagree with things you told Woolly directly. Review them with: woolly conflicts\n", conflicts)
	}
	return nil
}

func cmdProfile(args []string) error {
	fs := newFlags("profile")
	asJSON := fs.Bool("json", false, "print JSON")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	mems, err := a.store.List(false)
	if err != nil {
		return err
	}
	p := profile.Build(mems)
	if *asJSON {
		return printJSON(p)
	}
	fmt.Print(p.Markdown())
	return nil
}

func cmdConflicts(args []string) error {
	a, err := open()
	if err != nil {
		return err
	}
	mems, err := a.store.Conflicts()
	if err != nil {
		return err
	}
	if len(mems) == 0 {
		fmt.Println("No conflicts.")
		return nil
	}
	for _, m := range mems {
		fmt.Println(mcp.Line(m))
		if other, err := a.store.Get(m.ConflictsWith); err == nil {
			fmt.Println("   disagrees with:", mcp.Line(other))
		}
	}
	fmt.Println("\nKeep one with: woolly resolve <id>")
	return nil
}

func cmdResolve(args []string) error {
	id, err := oneID("resolve", args)
	if err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	kept, replaced, err := a.store.Resolve(id)
	if err != nil {
		return err
	}
	fmt.Println("Current fact:", kept.Content)
	for _, r := range replaced {
		fmt.Println("Replaced:", r.Content)
	}
	return nil
}

func cmdServe(args []string) error {
	fs := newFlags("serve")
	addr := fs.String("addr", envOr("WOOLLY_HOST", server.DefaultAddr), "listen address")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	token, err := server.EnsureToken(a.home)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Woolly %s listening on http://%s (memories: %s)\n", version, *addr, a.store.Dir())
	fmt.Fprintf(os.Stderr, "Open the review page with `woolly ui`. API token: %s\n", filepath.Join(a.home, "token"))
	return server.Serve(*addr, server.Handler(a.store, a.engine, version, token))
}

// cmdUI opens the review page, starting the server first if it isn't running.
func cmdUI(args []string) error {
	fs := newFlags("ui")
	addr := fs.String("addr", envOr("WOOLLY_HOST", server.DefaultAddr), "listen address")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	url := "http://" + *addr + "/"
	a, err := open()
	if err != nil {
		return err
	}
	token, err := server.EnsureToken(a.home)
	if err != nil {
		return err
	}
	link := url + "auth?token=" + token
	client := &http.Client{Timeout: time.Second}
	if res, err := client.Get(url + "api/version"); err == nil {
		res.Body.Close()
		fmt.Println("Opening", url)
		return openBrowser(link)
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("can't start the review page on %s: %w", *addr, err)
	}
	fmt.Printf("Woolly review page: %s\nPress Ctrl+C to stop.\n", url)
	_ = openBrowser(link)
	return server.ServeListener(ln, server.Handler(a.store, a.engine, version, token))
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func cmdProxy(args []string) error {
	fs := newFlags("proxy")
	addr := fs.String("addr", envOr("WOOLLY_PROXY_HOST", proxy.DefaultAddr), "listen address")
	ollamaURL := fs.String("ollama", "", "Ollama address (default: OLLAMA_HOST or 127.0.0.1:11434)")
	openaiURL := fs.String("openai", "", "OpenAI-compatible API for /v1 requests (default: Ollama)")
	anthropicURL := fs.String("anthropic", "https://api.anthropic.com", "Anthropic API address")
	limit := fs.Int("n", 6, "memories to add per request")
	noSave := fs.Bool("no-save", false, "don't save facts you state in chats")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	a, err := open()
	if err != nil {
		return err
	}
	parseURL := func(name, raw string) (*url.URL, error) {
		if !strings.Contains(raw, "://") {
			raw = "http://" + raw
		}
		u, err := url.Parse(strings.TrimRight(raw, "/"))
		if err != nil || u.Host == "" {
			return nil, fmt.Errorf("bad --%s address %q", name, raw)
		}
		return u, nil
	}
	ol := *ollamaURL
	if ol == "" {
		ol = a.ollama.Host
	}
	oa := *openaiURL
	if oa == "" {
		oa = ol
	}
	targets := map[string]*url.URL{}
	for name, raw := range map[string]string{"ollama": ol, "openai": oa, "anthropic": *anthropicURL} {
		u, err := parseURL(name, raw)
		if err != nil {
			return err
		}
		targets[name] = u
	}
	p := &proxy.Proxy{
		Store: a.store, Engine: a.engine,
		Ollama: targets["ollama"], OpenAI: targets["openai"], Anthropic: targets["anthropic"],
		Limit: *limit, Save: !*noSave,
		Log: log.New(os.Stderr, "woolly proxy: ", 0),
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("can't start the proxy on %s: %w", *addr, err)
	}
	host := strings.TrimPrefix(*addr, "http://")
	fmt.Fprintf(os.Stderr, `Woolly proxy on http://%s

  Ollama:     OLLAMA_HOST=%s ollama run llama3.2
  OpenAI:     set your app's base URL to http://%s/v1  (forwards to %s)
  Anthropic:  set your app's base URL to http://%s  (forwards to %s)

Memories are added to each chat. Facts you state are saved%s.
`, host, host, host, targets["openai"], host, targets["anthropic"], map[bool]string{true: "", false: " (off: --no-save)"}[p.Save])
	// No write timeout: model replies can stream for a long time.
	srv := &http.Server{Handler: server.LocalOnly(p.Handler()), ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

func cmdMCP(args []string) error {
	a, err := open()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	s := &mcp.Server{Store: a.store, Engine: a.engine, Version: version}
	return s.Serve(ctx, os.Stdin, os.Stdout)
}

func cmdDoctor(args []string) error {
	a, err := open()
	if err != nil {
		return err
	}
	all, err := a.store.List(true)
	if err != nil {
		return err
	}
	active, conflicts := 0, 0
	for _, m := range all {
		switch m.Status {
		case memory.StatusActive:
			active++
		case memory.StatusConflict:
			conflicts++
		}
	}
	fmt.Println("Woolly", version)
	fmt.Println("  memories:", a.store.Dir())
	fmt.Printf("  %d active, %d superseded, %d conflicts\n", active, len(all)-active-conflicts, conflicts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	v, err := a.ollama.Version(ctx)
	switch {
	case err != nil:
		fmt.Println("  search: keyword only (Ollama not running at " + a.ollama.Host + ")")
		fmt.Println("  tip: install Ollama from https://ollama.com, then: ollama pull " + a.ollama.EmbedModel)
	default:
		ok, err := a.ollama.HasModel(ctx, a.ollama.EmbedModel)
		switch {
		case err != nil:
			fmt.Println("  search: keyword only (could not list Ollama models:", err, ")")
		case !ok:
			fmt.Printf("  search: keyword only (Ollama %s is running, but %s isn't pulled)\n", v, a.ollama.EmbedModel)
			fmt.Println("  tip: ollama pull " + a.ollama.EmbedModel)
		default:
			fmt.Printf("  search: semantic + keyword (Ollama %s, %s)\n", v, a.ollama.EmbedModel)
		}
	}
	if conflicts > 0 {
		fmt.Println("  run 'woolly conflicts' to review disagreeing memories")
	}
	return nil
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

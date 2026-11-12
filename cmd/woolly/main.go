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

	"github.com/rjcuff/woolly/internal/activity"
	"github.com/rjcuff/woolly/internal/connect"
	"github.com/rjcuff/woolly/internal/importer"
	"github.com/rjcuff/woolly/internal/mcp"
	"github.com/rjcuff/woolly/internal/memory"
	"github.com/rjcuff/woolly/internal/ollama"
	"github.com/rjcuff/woolly/internal/peer"
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
  woolly remember <text> [--key k] [--tags a,b] [--project p] [--source user|agent|import] [--allow-secret]
  woolly learn [text]          pick out facts from text or a conversation with a local model
  woolly recall <query> [-n 5] [--project p] [--all] [--json]
  woolly list [--project p] [--all] [--json]
  woolly show <id>
  woolly forget <id>           move a memory to the trash
  woolly restore <id>          bring it back
  woolly trash [--empty]       list the trash, or delete it for good
  woolly import <chatgpt|claude> <export.zip> [--dry-run]
  woolly profile [--json]       summary of everything Woolly knows about you
  woolly conflicts
  woolly log [--app name] [-n 50]  what each app saved, read and deleted
  woolly context               print what Woolly knows, for an app to read at startup
  woolly resolve <id>          keep this memory as the current fact for its key
  woolly ui                    open the review page in your browser
  woolly connect [app...]      add Woolly to Claude, Cursor, VS Code, Codex, Gemini and more
  woolly disconnect [app...]   remove it again
  woolly sync [init|join|status]  keep memories in step between your devices
  woolly service install       run Woolly in the background, starting at login
  woolly service status|uninstall
  woolly serve [--proxy]       run the local HTTP API and review page (default 127.0.0.1:11435)
  woolly mcp                   run as an MCP server on stdio (for Claude, Cursor, ...)
  woolly proxy                 give memory to Ollama and OpenAI or Anthropic apps (127.0.0.1:11436)
  woolly update [--check]      update to the latest version
  woolly doctor                check your setup
  woolly version

Memories are plain Markdown files in ~/.woolly/memories (set WOOLLY_HOME to move them).
If Ollama is running, Woolly uses it for semantic search; otherwise keyword search.

Connect your AI apps:  woolly connect
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
	case "learn":
		return cmdLearn(rest)
	case "sync":
		return cmdSync(rest)
	case "log", "activity":
		return cmdLog(rest)
	case "context":
		return cmdContext(rest)
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
	case "update", "upgrade":
		return cmdUpdate(rest)
	case "connect":
		return cmdConnect(rest)
	case "disconnect":
		return cmdConnect(append([]string{"--remove"}, rest...))
	case "service":
		return cmdService(rest)
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
	log    *activity.Log
}

var importName = map[string]string{"chatgpt": "ChatGPT import", "claude": "Claude import"}

// cli is how the command line shows up in the activity log.
const cli = "command line"

func (a *app) record(action string, ids []string, count int) {
	a.log.Add(activity.Entry{App: cli, Action: action, IDs: ids, Count: count})
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
	return &app{home: h, store: store, engine: engine, ollama: oc, log: activity.Open(h)}, nil
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
	project := fs.String("project", "", "only for this project, like a repo name")
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
		Content: text, Key: *key, Tags: []string{*tags}, Source: src, AllowSensitive: *allowSecret, Via: cli, Project: *project,
	})
	var sensitive *memory.SensitiveError
	if errors.As(err, &sensitive) {
		return fmt.Errorf("%v. If you're sure, add --allow-secret", err)
	}
	if err != nil {
		return err
	}
	a.record("remember", []string{res.Memory.ID}, 0)
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
	project := fs.String("project", "", "search your memories plus this project's, leaving other projects out")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	query := strings.Join(pos, " ")
	a, err := open()
	if err != nil {
		return err
	}
	results, mode, err := a.engine.RecallIn(context.Background(), query, *n, *all, *project)
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
	project := fs.String("project", "", "only this project's memories")
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
	if p := memory.NormalizeProject(*project); p != "" {
		kept := mems[:0]
		for _, m := range mems {
			if m.Project == p {
				kept = append(kept, m)
			}
		}
		mems = kept
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
	a.record("forget", []string{m.ID}, 0)
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
	a.record("restore", []string{m.ID}, 0)
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
		a.record("empty_trash", nil, n)
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
			Via: importName[provider],
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
	a.log.Add(activity.Entry{App: importName[provider], Action: "import", Count: added})
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
	a.record("resolve", []string{kept.ID}, len(replaced))
	fmt.Println("Current fact:", kept.Content)
	for _, r := range replaced {
		fmt.Println("Replaced:", r.Content)
	}
	return nil
}

func cmdServe(args []string) error {
	fs := newFlags("serve")
	addr := fs.String("addr", envOr("WOOLLY_HOST", server.DefaultAddr), "listen address")
	withProxy := fs.Bool("proxy", false, "also run the proxy (see woolly proxy)")
	logPath := fs.String("log", "", "write output to this file instead of the terminal")
	var envs envFlags
	fs.Var(&envs, "env", "set a Woolly setting, e.g. --env WOOLLY_HOME=/path (used by the background service)")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if err := envs.apply(); err != nil {
		return err
	}
	if !flagSet(fs, "addr") {
		*addr = envOr("WOOLLY_HOST", server.DefaultAddr)
	}
	if *logPath != "" {
		if err := redirectOutput(*logPath); err != nil {
			return err
		}
	}
	a, err := open()
	if err != nil {
		return err
	}
	token, err := server.EnsureToken(a.home)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("can't start Woolly on %s (is it already running? try `woolly service status`): %w", *addr, err)
	}
	// Only the process that owns the port records its PID, so a second
	// `woolly serve` can't overwrite or delete the running server's file.
	if err := writePID(a.home); err != nil {
		return err
	}
	defer removePID(a.home)
	fmt.Fprintf(os.Stderr, "Woolly %s listening on http://%s (memories: %s)\n", version, *addr, a.store.Dir())
	fmt.Fprintf(os.Stderr, "Open the review page with `woolly ui`. API token: %s\n", filepath.Join(a.home, "token"))

	errc := make(chan error, 2)
	go func() {
		errc <- server.ServeListener(ln, server.Handler(a.store, a.engine, version, token, a.serverOptions()))
	}()
	if _, err := startSync(a, true); err != nil {
		fmt.Fprintln(os.Stderr, "woolly: sync is off:", err)
	}
	if *withProxy {
		srv, pln, err := startProxy(a, defaultProxyOptions())
		if err != nil {
			return err
		}
		go func() { errc <- srv.Serve(pln) }()
	}
	return <-errc
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
	return server.ServeListener(ln, server.Handler(a.store, a.engine, version, token, a.serverOptions()))
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

// proxyOptions configures the proxy. Empty addresses fall back to defaults.
type proxyOptions struct {
	addr, ollama, openai, anthropic string
	limit                           int
	save                            bool
}

func defaultProxyOptions() proxyOptions {
	return proxyOptions{
		addr:      envOr("WOOLLY_PROXY_HOST", proxy.DefaultAddr),
		anthropic: "https://api.anthropic.com",
		limit:     6,
		save:      true,
	}
}

func cmdProxy(args []string) error {
	o := defaultProxyOptions()
	fs := newFlags("proxy")
	fs.StringVar(&o.addr, "addr", o.addr, "listen address")
	fs.StringVar(&o.ollama, "ollama", "", "Ollama address (default: OLLAMA_HOST or 127.0.0.1:11434)")
	fs.StringVar(&o.openai, "openai", "", "OpenAI-compatible API for /v1 requests (default: Ollama)")
	fs.StringVar(&o.anthropic, "anthropic", o.anthropic, "Anthropic API address")
	fs.IntVar(&o.limit, "n", o.limit, "memories to add per request")
	noSave := fs.Bool("no-save", false, "don't save facts you state in chats")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	o.save = !*noSave
	a, err := open()
	if err != nil {
		return err
	}
	srv, ln, err := startProxy(a, o)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

// startProxy builds the proxy and opens its listener, printing how to use it.
func startProxy(a *app, o proxyOptions) (*http.Server, net.Listener, error) {
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
	ol := o.ollama
	if ol == "" {
		ol = a.ollama.Host
	}
	oa := o.openai
	if oa == "" {
		oa = ol
	}
	targets := map[string]*url.URL{}
	for name, raw := range map[string]string{"ollama": ol, "openai": oa, "anthropic": o.anthropic} {
		u, err := parseURL(name, raw)
		if err != nil {
			return nil, nil, err
		}
		targets[name] = u
	}
	p := &proxy.Proxy{
		Store: a.store, Engine: a.engine,
		Ollama: targets["ollama"], OpenAI: targets["openai"], Anthropic: targets["anthropic"],
		Limit: o.limit, Save: o.save,
		Log:      log.New(os.Stderr, "woolly proxy: ", 0),
		Activity: a.log,
		Learner:  a.learner(),
	}
	ln, err := net.Listen("tcp", o.addr)
	if err != nil {
		return nil, nil, fmt.Errorf("can't start the proxy on %s: %w", o.addr, err)
	}
	host := strings.TrimPrefix(o.addr, "http://")
	fmt.Fprintf(os.Stderr, `Woolly proxy on http://%s

  Ollama:     OLLAMA_HOST=%s ollama run llama3.2
  OpenAI:     set your app's base URL to http://%s/v1  (forwards to %s)
  Anthropic:  set your app's base URL to http://%s  (forwards to %s)

Memories are added to each chat. Facts you state are saved%s.
`, host, host, host, targets["openai"], host, targets["anthropic"], map[bool]string{true: "", false: " (off: --no-save)"}[p.Save])
	// No write timeout: model replies can stream for a long time.
	srv := &http.Server{Handler: server.LocalOnly(p.Handler()), ReadHeaderTimeout: 10 * time.Second}
	return srv, ln, nil
}

func cmdMCP(args []string) error {
	a, err := open()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	s := &mcp.Server{Store: a.store, Engine: a.engine, Version: version, Activity: a.log, Project: mcpProject()}
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
	if ok, err := a.ollama.HasModel(ctx, a.ollama.ChatModel); err == nil && ok {
		fmt.Printf("  woolly learn: ready (%s)\n", a.ollama.ChatModel)
	} else if err == nil {
		fmt.Printf("  woolly learn: off (run: ollama pull %s)\n", a.ollama.ChatModel)
	}

	fmt.Println("\nSync")
	if cfg, err := peer.Load(a.home); err != nil {
		fmt.Println("  error:", err)
	} else if cfg == nil {
		fmt.Println("  off (set it up with: woolly sync init)")
	} else {
		fmt.Printf("  on, %d linked devices (see woolly sync status)\n", len(cfg.Peers))
	}

	fmt.Println("\nBackground service")
	if spec, _, err := serviceSpec(); err == nil {
		fmt.Println(serviceSummary(spec))
	}

	fmt.Println("\nApps")
	connected, missing := connectedApps()
	if len(connected) > 0 {
		fmt.Println("  connected:", strings.Join(connected, ", "))
	}
	if len(missing) > 0 {
		fmt.Println("  not connected:", strings.Join(missing, ", "))
		fmt.Println("  tip: woolly connect")
	}
	if len(connected)+len(missing) == 0 {
		fmt.Println("  no MCP apps found (Claude Code is checked by `woolly connect`)")
	}
	if h, err := os.UserHomeDir(); err == nil && connect.HasStartupHook(connect.ClaudeSettings(h)) {
		fmt.Println("  Claude Code loads your profile at startup")
	}
	if n, _ := a.log.Recent(0, ""); len(n) > 0 {
		fmt.Printf("\nActivity: %d entries in %s (see `woolly log`)\n", len(n), a.log.Path())
	}
	return nil
}

func envOr(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

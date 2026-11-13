package share

import (
	"regexp"
	"sort"
	"strings"

	"github.com/rjcuff/woolly/internal/memory"
)

// The repo's project memory is .woolly/woolly.md: one fact per line under a
// few headings, sorted, so diffs stay small and merges rarely conflict.

// Section is a heading in woolly.md and the tag its memories carry.
type Section struct{ Name, Tag string }

// Sections are the headings every woolly.md has, in order.
var Sections = []Section{
	{"Overview", "overview"},
	{"Stack", "stack"},
	{"Conventions", "convention"},
	{"Decisions", "decision"},
	{"Gotchas", "gotcha"},
	{"Commands", "command"},
}

// SectionNames lists every heading, Notes last.
func SectionNames() []string {
	var out []string
	for _, s := range Sections {
		out = append(out, s.Name)
	}
	return append(out, Notes)
}

// Notes holds facts that fit no other section. It only shows when it has some.
const Notes = "Notes"

const intro = `What AI tools have learned about this project, kept by Woolly (https://usewoolly.com).
One fact per line. Add, edit or delete lines and Woolly picks up the change.
The comment at the end of a line is how Woolly keeps track of it.`

// Config is the settings at the top of woolly.md.
type Config struct {
	Project string
	Export  []string // files to keep a Woolly section in: "claude", "agents"
	Capture bool     // save decisions when a Claude Code session ends
}

// Exports reports whether the config keeps a section in file ("claude" or "agents").
func (c Config) Exports(file string) bool {
	for _, e := range c.Export {
		if e == file {
			return true
		}
	}
	return false
}

type line struct {
	section string
	text    string // the fact
	id      string // last part of the memory's ID
	key, by string
	raw     string // the line as written, for lines Woolly doesn't manage
}

type doc struct {
	cfg   Config
	intro string
	lines []line
}

func newDoc(project string) doc {
	return doc{cfg: Config{Project: project, Capture: true}, intro: intro}
}

var (
	attrRe = regexp.MustCompile(`(\w+)=("[^"]*"|\S+)`)
	idRe   = regexp.MustCompile(`^[0-9a-f]{8}$`)
)

func parseDoc(text string) doc {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	d := doc{cfg: Config{Capture: true}}
	body := text
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---"); end >= 0 {
			for _, l := range strings.Split(text[4:4+end], "\n") {
				k, v, ok := strings.Cut(l, ":")
				if !ok {
					continue
				}
				v = strings.TrimSpace(v)
				switch strings.TrimSpace(k) {
				case "project":
					d.cfg.Project = memory.NormalizeProject(v)
				case "export":
					for _, e := range strings.Split(v, ",") {
						if e = strings.ToLower(strings.TrimSpace(e)); e == "claude" || e == "agents" {
							d.cfg.Export = append(d.cfg.Export, e)
						}
					}
				case "capture":
					d.cfg.Capture = v != "off" && v != "false" && v != "no"
				}
			}
			body = text[4+end+4:]
		}
	}
	section := ""
	var head []string
	for _, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "## ") {
			section = canonical(t[3:])
			continue
		}
		if section == "" {
			if !strings.HasPrefix(t, "# ") {
				head = append(head, l)
			}
			continue
		}
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			ln := parseFact(t[2:])
			ln.section = section
			if ln.text != "" {
				d.lines = append(d.lines, ln)
			}
			continue
		}
		d.lines = append(d.lines, line{section: section, raw: l})
	}
	d.intro = strings.TrimSpace(strings.Join(head, "\n"))
	return d
}

func parseFact(s string) line {
	var ln line
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "<!--"); i >= 0 && strings.HasSuffix(s, "-->") {
		for _, m := range attrRe.FindAllStringSubmatch(s[i+4:len(s)-3], -1) {
			v := strings.Trim(m[2], `"`)
			switch m[1] {
			case "id":
				if idRe.MatchString(v) {
					ln.id = v
				}
			case "key":
				ln.key = memory.NormalizeKey(v)
			case "by":
				ln.by = memory.CleanApp(v)
			}
		}
		s = strings.TrimSpace(s[:i])
	}
	ln.text = oneLine(s)
	return ln
}

func canonical(heading string) string {
	h := strings.ToLower(strings.TrimSpace(heading))
	for _, s := range Sections {
		if n := strings.ToLower(s.Name); h == n || h == strings.TrimSuffix(n, "s") {
			return s.Name
		}
	}
	return Notes
}

// oneLine keeps a fact on one line and out of the tracking comment.
func oneLine(s string) string {
	s = strings.NewReplacer("<!--", "", "-->", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func (ln line) String() string {
	if ln.raw != "" {
		return ln.raw
	}
	attrs := []string{"id=" + ln.id}
	if ln.key != "" {
		attrs = append(attrs, "key="+ln.key)
	}
	if by := strings.ReplaceAll(ln.by, `"`, ""); by != "" {
		if strings.ContainsAny(by, " \t") {
			by = `"` + by + `"`
		}
		attrs = append(attrs, "by="+by)
	}
	return "- " + ln.text + " <!-- " + strings.Join(attrs, " ") + " -->"
}

func (d doc) render() string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("project: " + d.cfg.Project + "\n")
	if len(d.cfg.Export) > 0 {
		b.WriteString("export: " + strings.Join(d.cfg.Export, ", ") + "\n")
	}
	if d.cfg.Capture {
		b.WriteString("capture: on\n")
	} else {
		b.WriteString("capture: off\n")
	}
	b.WriteString("---\n\n# Project memory\n\n")
	if d.intro != "" {
		b.WriteString(d.intro + "\n\n")
	}
	for _, name := range SectionNames() {
		raw, facts := d.in(name)
		if name == Notes && len(raw)+len(facts) == 0 {
			continue
		}
		b.WriteString("## " + name + "\n\n")
		for _, ln := range append(raw, facts...) {
			b.WriteString(ln.String() + "\n")
		}
		if len(raw)+len(facts) > 0 {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// in returns a section's lines: the ones kept as written, then the facts,
// sorted by key and then by text.
func (d doc) in(section string) (raw, facts []line) {
	for _, ln := range d.lines {
		if ln.section != section {
			continue
		}
		if ln.raw != "" {
			raw = append(raw, ln)
		} else {
			facts = append(facts, ln)
		}
	}
	sort.SliceStable(facts, func(i, j int) bool {
		a, b := facts[i], facts[j]
		if (a.key == "") != (b.key == "") {
			return a.key != ""
		}
		if a.key != b.key {
			return a.key < b.key
		}
		return strings.ToLower(a.text) < strings.ToLower(b.text)
	})
	return raw, facts
}

// suffix is the random last part of a memory ID, which woolly.md uses.
func suffix(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// SectionOf is the woolly.md heading a memory goes under: its section tag,
// or a guess from the words in it.
func SectionOf(m memory.Memory) string {
	for _, s := range Sections {
		for _, t := range m.Tags {
			if t == s.Tag {
				return s.Name
			}
		}
	}
	return Guess(m.Content)
}

// TagFor is the tag for a heading, or "" for Notes.
func TagFor(section string) string {
	for _, s := range Sections {
		if s.Name == section {
			return s.Tag
		}
	}
	return ""
}

func withSection(tags []string, section string) []string {
	var out []string
	for _, t := range tags {
		isSection := false
		for _, s := range Sections {
			isSection = isSection || s.Tag == t
		}
		if !isSection {
			out = append(out, t)
		}
	}
	if t := TagFor(section); t != "" {
		out = append(out, t)
	}
	return memory.NormalizeTags(out)
}

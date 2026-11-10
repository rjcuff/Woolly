// Package importer pulls facts about the user out of chat history exported
// from ChatGPT and Claude.
//
// It only looks at what the user wrote, and only keeps short, plain
// statements about themselves ("I live in Denver", "I prefer tabs",
// "remember that my sister's name is Ana"). Questions, code and long
// messages are skipped. Nothing leaves the machine.
package importer

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Message is one thing the user said.
type Message struct {
	Text string
	Time time.Time
}

// Candidate is a fact found in the history.
type Candidate struct {
	Content string
	Key     string
	Tags    []string
	Time    time.Time
}

// Providers lists the supported exports.
var Providers = []string{"chatgpt", "claude"}

// Load reads user messages from an export file. path may be the export
// .zip or the conversations.json inside it. Messages come back oldest first.
func Load(provider, path string) ([]Message, error) {
	data, err := readConversations(path)
	if err != nil {
		return nil, err
	}
	var msgs []Message
	switch provider {
	case "chatgpt":
		msgs, err = parseChatGPT(data)
	case "claude":
		msgs, err = parseClaude(data)
	default:
		return nil, fmt.Errorf("unknown source %q (use %s)", provider, strings.Join(Providers, " or "))
	}
	if err != nil {
		return nil, fmt.Errorf("read %s export: %w", provider, err)
	}
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Time.Before(msgs[j].Time) })
	return msgs, nil
}

func readConversations(path string) ([]byte, error) {
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return os.ReadFile(path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.Base(f.Name) != "conversations.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		// Real exports are well under this; it stops a malicious zip from
		// filling memory.
		const maxSize = 2 << 30
		data, err := io.ReadAll(io.LimitReader(rc, maxSize+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxSize {
			return nil, errors.New("conversations.json is larger than 2 GB")
		}
		return data, nil
	}
	return nil, errors.New("no conversations.json in the zip")
}

// ChatGPT: [{ "mapping": { id: { "message": { "author": {"role"}, "content": {"parts"}, "create_time" } } } }]
func parseChatGPT(data []byte) ([]Message, error) {
	var convs []struct {
		Mapping map[string]struct {
			Message *struct {
				Author struct {
					Role string `json:"role"`
				} `json:"author"`
				Content struct {
					ContentType string            `json:"content_type"`
					Parts       []json.RawMessage `json:"parts"`
				} `json:"content"`
				CreateTime *float64 `json:"create_time"`
			} `json:"message"`
		} `json:"mapping"`
	}
	if err := json.Unmarshal(data, &convs); err != nil {
		return nil, err
	}
	var out []Message
	for _, c := range convs {
		for _, node := range c.Mapping {
			m := node.Message
			if m == nil || m.Author.Role != "user" || m.Content.ContentType != "text" {
				continue
			}
			var parts []string
			for _, p := range m.Content.Parts {
				var s string
				if json.Unmarshal(p, &s) == nil {
					parts = append(parts, s)
				}
			}
			msg := Message{Text: strings.Join(parts, "\n")}
			if m.CreateTime != nil {
				sec := int64(*m.CreateTime)
				msg.Time = time.Unix(sec, int64((*m.CreateTime-float64(sec))*1e9))
			}
			out = append(out, msg)
		}
	}
	return out, nil
}

// Claude: [{ "chat_messages": [{ "sender": "human", "text", "content": [{"type","text"}], "created_at" }] }]
func parseClaude(data []byte) ([]Message, error) {
	var convs []struct {
		ChatMessages []struct {
			Sender    string `json:"sender"`
			Text      string `json:"text"`
			CreatedAt string `json:"created_at"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"chat_messages"`
	}
	if err := json.Unmarshal(data, &convs); err != nil {
		return nil, err
	}
	var out []Message
	for _, c := range convs {
		for _, m := range c.ChatMessages {
			if m.Sender != "human" {
				continue
			}
			text := m.Text
			if text == "" {
				var parts []string
				for _, p := range m.Content {
					if p.Type == "text" {
						parts = append(parts, p.Text)
					}
				}
				text = strings.Join(parts, "\n")
			}
			t, _ := time.Parse(time.RFC3339Nano, m.CreatedAt)
			out = append(out, Message{Text: text, Time: t})
		}
	}
	return out, nil
}

type rule struct {
	re   *regexp.Regexp
	key  func(m []string) string
	tags []string
}

func fixed(k string) func([]string) string { return func([]string) string { return k } }

var rules = []rule{
	{re: regexp.MustCompile(`(?i)^my name is\s+([\p{L}' -]{2,40})$`), key: fixed("user.name"), tags: []string{"identity"}},
	{re: regexp.MustCompile(`(?i)^(?:i live in|i'm based in|i am based in|i moved to|i just moved to)\s+(.{2,60})$`),
		key: fixed("user.location"), tags: []string{"home"}},
	{re: regexp.MustCompile(`(?i)^i work (?:at|for)\s+(.{2,60})$`), key: fixed("user.employer"), tags: []string{"work"}},
	{re: regexp.MustCompile(`(?i)^i work as an?\s+(.{2,60})$`), key: fixed("user.role"), tags: []string{"work"}},
	{re: regexp.MustCompile(`(?i)^my favorite\s+([\p{L} ]{2,30}?)\s+is\s+(.{1,60})$`),
		key: func(m []string) string { return "favorite." + strings.ToLower(strings.TrimSpace(m[1])) }, tags: []string{"preference"}},
	{re: regexp.MustCompile(`(?i)^i (?:really )?(?:prefer|like|love|enjoy)\s+(.{2,100})$`), tags: []string{"preference"}},
	{re: regexp.MustCompile(`(?i)^i (?:really )?(?:don't like|do not like|dislike|hate|avoid)\s+(.{2,100})$`), tags: []string{"preference"}},
	{re: regexp.MustCompile(`(?i)^i (?:mostly |usually |always )?use\s+(.{2,80})$`), tags: []string{"tools"}},
	{re: regexp.MustCompile(`(?i)^i(?:'m| am) allergic to\s+(.{2,60})$`), key: fixed("health.allergies"), tags: []string{"health"}},
}

var (
	sentenceEnd = regexp.MustCompile(`[.!\n]+\s*`)
	andClause   = regexp.MustCompile(`(?i),?\s+and\s+(?:also\s+)?(i|my)\s`)
	remember    = regexp.MustCompile(`(?i)^(?:please\s+)?remember(?:\s+that)?\s+(.{3,})$`)
	vague       = regexp.MustCompile(`(?i)\b(this|that|it|these|those|the code|the following|above|below|your)\s*$`)
	leadIn      = regexp.MustCompile(`(?i)^(?:(?:so|also|and|oh|btw|fyi|actually|by the way|just so you know|for context|for reference|update)[,:]?\s+)+`)
)

// clean drops short lead-ins like "Quick update:" or "Also," so the rules
// can match the statement itself.
func clean(s string) string {
	if i := strings.LastIndex(s, ": "); i >= 0 && i < 30 {
		s = s[i+2:]
	}
	return leadIn.ReplaceAllString(s, "")
}

// Extract finds facts in messages. Later messages win, so if you moved
// twice, the newest location is the one that stays current.
func Extract(msgs []Message) []Candidate {
	var out []Candidate
	seen := map[string]bool{}
	for _, msg := range msgs {
		if len(msg.Text) > 2000 || strings.Contains(msg.Text, "```") {
			continue // long pastes and code are not about the user
		}
		for _, s := range sentences(msg.Text) {
			s = clean(strings.TrimRight(s, ",;: "))
			if len(s) < 8 || len(s) > 160 || strings.ContainsAny(s, "?`{}<>") || vague.MatchString(s) {
				continue
			}
			// "Remember that ..." is always kept, and still gets a key if
			// what follows matches one of the rules.
			noted := false
			if m := remember.FindStringSubmatch(s); m != nil {
				s, noted = capitalize(strings.TrimSpace(m[1])), true
			}
			c := Candidate{Content: capitalize(s), Time: msg.Time}
			matched := false
			for _, r := range rules {
				m := r.re.FindStringSubmatch(s)
				if m == nil {
					continue
				}
				c.Tags = r.tags
				if r.key != nil {
					c.Key = r.key(m)
				}
				matched = true
				break
			}
			if !matched && !noted {
				continue
			}
			if noted {
				c.Tags = append([]string{"noted"}, c.Tags...)
			}
			id := strings.ToLower(c.Key + "|" + c.Content)
			if !seen[id] {
				seen[id] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// sentences splits a message into short statements, including clauses
// joined by "and I" or "and my".
func sentences(text string) []string {
	var out []string
	for _, s := range sentenceEnd.Split(text, -1) {
		s = strings.Join(strings.Fields(s), " ")
		for {
			loc := andClause.FindStringSubmatchIndex(s)
			if loc == nil {
				break
			}
			out = append(out, strings.TrimSpace(s[:loc[0]]))
			s = s[loc[2]:] // keep the "I" or "my"
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

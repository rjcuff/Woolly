// Package secrets spots things that should never be saved as a memory:
// API keys, tokens, private keys, passwords, card numbers and the like.
package secrets

import (
	"regexp"
	"strings"
)

// Finding names one kind of sensitive data found in a piece of text.
type Finding struct {
	Kind string // e.g. "OpenAI API key"
}

type pattern struct {
	kind string
	re   *regexp.Regexp
}

var patterns = []pattern{
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"AWS access key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"GitHub token", regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{60,})\b`)},
	{"Anthropic API key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
	{"OpenAI API key", regexp.MustCompile(`\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}`)},
	{"Stripe key", regexp.MustCompile(`\b(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{20,}\b`)},
	{"Slack token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"Google API key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"JSON web token", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"password", regexp.MustCompile(`(?i)\b(?:password|passwd|passcode|pwd)\b\s*(?:is|=|:)\s*\S{4,}`)},
	{"API key", regexp.MustCompile(`(?i)\b(?:api[_ -]?key|secret[_ -]?key|access[_ -]?token|auth[_ -]?token)\b\s*(?:is|=|:)\s*\S{12,}`)},
	{"Social Security number", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)},
}

var cardCandidate = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)

// Scan returns every kind of sensitive data found in text, without
// repeating a kind.
func Scan(text string) []Finding {
	var out []Finding
	seen := map[string]bool{}
	add := func(kind string) {
		if !seen[kind] {
			seen[kind] = true
			out = append(out, Finding{Kind: kind})
		}
	}
	for _, p := range patterns {
		if p.kind == "OpenAI API key" && seen["Anthropic API key"] {
			continue // sk-ant- keys also look like OpenAI keys
		}
		if p.re.MatchString(text) {
			add(p.kind)
		}
	}
	for _, m := range cardCandidate.FindAllString(text, -1) {
		if luhn(m) {
			add("card number")
		}
	}
	return out
}

// Describe joins findings into a short phrase like "an OpenAI API key".
func Describe(f []Finding) string {
	kinds := make([]string, len(f))
	for i, x := range f {
		kinds[i] = article(x.Kind) + " " + x.Kind
	}
	switch len(kinds) {
	case 0:
		return ""
	case 1:
		return kinds[0]
	}
	return strings.Join(kinds[:len(kinds)-1], ", ") + " and " + kinds[len(kinds)-1]
}

func article(s string) string {
	if s != "" && strings.ContainsRune("AEIOU", rune(strings.ToUpper(s)[0])) {
		return "an"
	}
	return "a"
}

// luhn reports whether a string of digits (with optional spaces or dashes)
// passes the Luhn check used by card numbers.
func luhn(s string) bool {
	var digits []int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits = append(digits, int(r-'0'))
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

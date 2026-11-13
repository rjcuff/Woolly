package share

import (
	"regexp"
	"strings"
)

func words(list ...string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(` + strings.Join(list, "|") + `)\b`)
}

var (
	commandRe = regexp.MustCompile("(?i)(^run |`[^`]*\\b(make|npm|pnpm|yarn|bun|go|cargo|pytest|python3?|docker|uv|just|git)\\b[^`]*`|`[a-z][\\w.-]* [^`]+`)")
	gotchaRe  = words("never", "don't", "do not", "avoid", "careful", "watch out", "breaks", "broken", "fails", "gotcha", "beware", "must not", "flaky", "workaround", "bug")
	stackRe   = words("uses", "built with", "written in", "runs on", "database", "framework", "postgres", "postgresql", "mysql", "sqlite", "redis", "react", "next.js", "vue", "svelte", "django", "rails", "go", "rust", "typescript", "python", "node", "tailwind", "vercel", "aws", "docker", "kubernetes")
	convRe    = words("always", "prefer", "should", "style", "naming", "convention", "format", "use", "instead of", "keep", "write")
	overRe    = words("is a", "is an", "this project", "this repo", "the app", "purpose")

	// Things about a person, which never go in a repo.
	personalRe = words("i", "i'm", "i am", "my", "me", "mine", "the user", "user's", "lives in", "live in", "born", "birthday", "years old", "wife", "husband", "partner", "kids", "children", "my name", "name is", "email address", "phone", "home address", "favorite", "hobby", "hobbies", "works at", "my job", "salary", "allergic", "vegetarian", "vegan")
	// Things about a codebase.
	projectRe = words("repo", "codebase", "this project", "build", "builds", "test", "tests", "deploy", "deploys", "endpoint", "api", "database", "schema", "migration", "migrations", "config", "ci", "branch", "package", "module", "service", "function", "component", "server", "client", "port", "env", "dependency", "dependencies", "lint", "release", "version")
	pathRe    = regexp.MustCompile("`[^`]+`|\\b[\\w-]+/[\\w./-]+|\\b[\\w-]+\\.(go|ts|tsx|js|py|rs|md|json|yml|yaml|toml|sql)\\b")
)

// Guess picks a heading for a fact from its words.
func Guess(content string) string {
	c := strings.TrimSpace(content)
	switch {
	case strings.HasPrefix(c, "Decided:"):
		return "Decisions"
	case commandRe.MatchString(c):
		return "Commands"
	case gotchaRe.MatchString(c):
		return "Gotchas"
	case stackRe.MatchString(c):
		return "Stack"
	case convRe.MatchString(c):
		return "Conventions"
	case overRe.MatchString(c):
		return "Overview"
	}
	return Notes
}

// LooksPersonal reports whether a fact is about a person rather than code.
func LooksPersonal(content string) bool { return personalRe.MatchString(content) }

// LooksProject reports whether a fact is about a codebase.
func LooksProject(content string) bool {
	return projectRe.MatchString(content) || pathRe.MatchString(content) || Guess(content) == "Commands"
}

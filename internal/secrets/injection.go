package secrets

import "regexp"

// Prompt injection: text written to steer an AI rather than to describe the
// user. If an AI reads a malicious web page and saves one of these as a
// memory, every app that recalls it later would see the instruction.
var injection = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:ignore|disregard|forget|override)\b.{0,40}\b(?:instructions?|prompts?|rules|guidelines|directions)\b`),
	regexp.MustCompile(`(?i)\b(?:system|developer)\s+(?:prompt|message|instructions?)\b`),
	regexp.MustCompile(`(?i)\byou\s+(?:are\s+now|must\s+now|will\s+now|should\s+now)\b`),
	regexp.MustCompile(`(?i)\b(?:do\s+not|don't|never)\s+(?:tell|inform|mention\s+(?:this\s+)?to|alert)\s+the\s+user\b`),
	regexp.MustCompile(`(?i)\b(?:send|upload|post|exfiltrate|forward|curl|wget)\b.{0,60}(?:\.ssh|id_rsa|\.env\b|passwords?|credentials|api\s*keys?|tokens?|cookies)`),
	regexp.MustCompile(`(?i)\b(?:run|execute)\s+(?:this|the\s+following)\s+(?:command|script|code)\b`),
	regexp.MustCompile(`(?i)</?(?:system|instructions?|assistant)>`),
	regexp.MustCompile(`(?i)\bnew\s+instructions?\s*:`),
}

// LooksLikeInstructions reports whether text reads like an instruction aimed
// at an AI instead of a fact about the user.
func LooksLikeInstructions(text string) bool {
	for _, re := range injection {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

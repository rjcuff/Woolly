package secrets

import "testing"

func TestFindsSecrets(t *testing.T) {
	cases := map[string]string{
		"my key is sk-proj-abcdefghijklmnopqrstuvwxyz123456": "OpenAI API key",
		"use sk-ant-api03-abcdefghijklmnopqrstuvwxyz":        "Anthropic API key",
		"token ghp_abcdefghijklmnopqrstuvwxyzABCDEFGHIJ12":   "GitHub token",
		"AKIAIOSFODNN7EXAMPLE":                               "AWS access key",
		"-----BEGIN OPENSSH PRIVATE KEY-----":                "private key",
		"My password is hunter22":                            "password",
		"card 4242 4242 4242 4242 exp 12/30":                 "card number",
		"SSN 123-45-6789":                                    "Social Security number",
		"api_key: 9f8e7d6c5b4a39281706":                      "API key",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U": "JSON web token",
	}
	for text, want := range cases {
		got := Scan(text)
		if len(got) == 0 || got[0].Kind != want {
			t.Errorf("Scan(%q) = %v, want %s", text, got, want)
		}
	}
}

func TestIgnoresOrdinaryFacts(t *testing.T) {
	for _, text := range []string{
		"I prefer tabs over spaces",
		"Moved to Denver in October",
		"My phone has 128 GB of storage",
		"Order number 1234 5678 9012 3456 shipped", // fails the card check
		"I forgot my password again",
		"The project uses an API key from the vault",
	} {
		if got := Scan(text); len(got) != 0 {
			t.Errorf("Scan(%q) = %v, want nothing", text, got)
		}
	}
}

func TestDescribe(t *testing.T) {
	got := Describe([]Finding{{"OpenAI API key"}, {"password"}})
	if got != "an OpenAI API key and a password" {
		t.Fatalf("got %q", got)
	}
}

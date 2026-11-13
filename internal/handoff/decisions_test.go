package handoff

import (
	"context"
	"testing"
)

func TestDecisions(t *testing.T) {
	reply := `{"decisions":[
		{"choice":"Use SQLite instead of Postgres","why":"one file, no server","rejected":["Postgres: too much setup"]},
		{"choice":"Added Node's type definitions","why":"for tests","rejected":[]},
		{"choice":"Keep the API under /v1","why":"","rejected":[]}
	]}`
	got, err := Decisions(context.Background(), fakeModel{reply}, []Turn{{Role: "user", Text: "let's use sqlite"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Choice != "Use SQLite instead of Postgres" || len(got[0].Rejected) != 1 {
		t.Fatalf("only real decisions with a reason: %+v", got)
	}
}

func TestSimilar(t *testing.T) {
	if !Similar("Decided: Use SQLite instead of Postgres. Why: one file.", "Decided: use SQLite, not Postgres. Why: one file") {
		t.Error("same decision in other words")
	}
	if Similar("Decided: Use SQLite. Why: one file.", "Decided: Keep the API under /v1. Why: stable URLs.") {
		t.Error("different decisions")
	}
}

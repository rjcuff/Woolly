package main

import "testing"

func TestClosest(t *testing.T) {
	cases := map[string]string{"remmber": "remember", "projct": "project", "recal": "recall", "zzzzzz": "", "int": "init"}
	for typo, want := range cases {
		if got := closest(typo); got != want {
			t.Errorf("closest(%q) = %q, want %q", typo, got, want)
		}
	}
}

package llm

import (
	"strings"
	"testing"
)

func TestLanguageInstruction(t *testing.T) {
	cases := []struct {
		code string
		want string // substring that must be present; "" means empty result
	}{
		{code: "", want: ""},
		{code: "en", want: ""},
		{code: "EN", want: ""},
		{code: "es", want: "Spanish"},
		{code: "fr", want: "French"},
		{code: "xx", want: `ISO 639-1 code "xx"`},
	}
	for _, c := range cases {
		got := LanguageInstruction(c.code)
		if c.want == "" {
			if got != "" {
				t.Fatalf("LanguageInstruction(%q) = %q, want empty", c.code, got)
			}
			continue
		}
		if !strings.Contains(got, c.want) {
			t.Fatalf("LanguageInstruction(%q) = %q, want substring %q", c.code, got, c.want)
		}
	}
}

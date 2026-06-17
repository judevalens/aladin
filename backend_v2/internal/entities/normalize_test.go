package entities

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"OpenAI", "openai"},
		{"OpenAI Inc", "openai"},
		{"OpenAI, Inc.", "openai"},
		{"openai", "openai"},
		{"  OpenAI   Inc.  ", "openai"},
		{"The New York Times", "new york times"},
		{"Acme Corp", "acme"},
		{"Acme Corporation", "acme"},
		{"Foo LLC", "foo"},
		{"", ""},
		{"!!!", ""},
		{"Inc", "inc"}, // a lone suffix word is kept, never stripped to empty
		{"Café", "café"}, // diacritics are preserved in R0 (folding deferred)
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

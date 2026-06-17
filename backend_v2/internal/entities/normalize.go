// Package entities implements entity resolution — turning the free-text entity
// mentions on records into a canonical entity registry with stable ids. R0 is the
// foundation: a deterministic normalizer + exact-key resolve-or-create over the
// shared (Tier 0) scope. Fuzzy matching, merges, embeddings, the tenant tier, and
// same-name/same-kind splitting are later phases.
package entities

import (
	"strings"
	"unicode"
)

// orgSuffixes are trailing legal/structural tokens dropped during normalization so
// "OpenAI", "OpenAI Inc", and "OpenAI, Inc." collapse to one key. R0 strips them
// kind-agnostically (this branch's enrichment carries no entity type yet); kind-aware
// stripping is a refinement for when typed entities land.
var orgSuffixes = map[string]bool{
	"inc": true, "incorporated": true, "llc": true, "llp": true,
	"ltd": true, "limited": true, "corp": true, "corporation": true,
	"co": true, "company": true, "gmbh": true, "plc": true, "pbc": true,
	"sa": true, "ag": true, "nv": true, "ab": true, "oy": true, "kk": true,
	"srl": true, "bv": true,
}

// Normalize reduces a surface form to a blocking key: lowercased, punctuation
// flattened to spaces, a leading "the" dropped, and trailing org suffixes removed.
// It is deterministic and dependency-free. Returns "" for an empty or
// punctuation-only surface. (Diacritic folding is deliberately deferred to a later
// phase; é stays é.)
func Normalize(surface string) string {
	var b strings.Builder
	b.Grow(len(surface))
	for _, r := range strings.ToLower(strings.TrimSpace(surface)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	fields := strings.Fields(b.String())
	if len(fields) > 1 && fields[0] == "the" {
		fields = fields[1:]
	}
	// Drop trailing org suffixes, but never the only token (so "Inc" alone stays "inc").
	for len(fields) > 1 && orgSuffixes[fields[len(fields)-1]] {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

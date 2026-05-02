package commons_invoice

import "strings"

// status_groups.go — synonym mapping for invoice status filtering.
//
// The canonical statuses are defined in status.go (draft / sent / paid /
// overdue / void). Users (and language models acting on their behalf)
// rarely speak in canonical terms; they say "open invoices", "pending",
// "unpaid", "outstanding". This file resolves those synonyms to canonical
// sets so callers can pass natural-language status filters and get
// consistent results.
//
// The mapping is single-source-of-truth for the codebase:
//   - chat tool runners use it to expand model-supplied terms
//   - the system prompt in prompt_engine references the same set
//   - any future REST/UI filters can plug in by calling ResolveStatuses
//
// Both keys and the canonical strings are lowercase. Inputs are
// trimmed + lowercased before matching, so casing from callers doesn't
// matter.

// StatusSynonyms maps user-friendly meta-words to canonical status sets.
// Keys are lowercase. Values are canonical Status strings — same format
// the DB stores, matches the FSM in status.go.
//
// Notes on the choices:
//   - "active" and "open" both span draft → sent → overdue. They mean
//     "the invoice is still in play", which excludes paid and void.
//   - "outstanding" / "unpaid" deliberately omit draft. Drafts haven't
//     been sent yet, so the customer doesn't owe anything. This matches
//     standard accounting usage.
//   - "pending" is draft + sent — work that needs to happen, but isn't
//     overdue yet.
//   - "closed" / "settled" / "done" are the terminals: paid + void.
var StatusSynonyms = map[string][]Status{
	"active":      {StatusDraft, StatusSent, StatusOverdue},
	"open":        {StatusDraft, StatusSent, StatusOverdue},
	"in-progress": {StatusDraft, StatusSent, StatusOverdue},
	"in_progress": {StatusDraft, StatusSent, StatusOverdue},
	"unpaid":      {StatusSent, StatusOverdue},
	"outstanding": {StatusSent, StatusOverdue},
	"owed":        {StatusSent, StatusOverdue},
	"due":         {StatusSent, StatusOverdue},
	"pending":     {StatusDraft, StatusSent},
	"closed":      {StatusPaid, StatusVoid},
	"settled":     {StatusPaid, StatusVoid},
	"done":        {StatusPaid, StatusVoid},
	"finalized":   {StatusPaid, StatusVoid},
}

// ResolveStatuses takes a mixed list of status strings (canonical
// statuses, synonyms, or both) and returns the de-duplicated canonical
// set as plain strings — exactly the shape SearchFilters.Statuses wants.
// Unknown values are dropped silently; the model occasionally hallucinates
// terms and we'd rather return a sane filter than error out.
//
// Empty/whitespace-only input returns nil (no filter).
func ResolveStatuses(inputs []string) []string {
	if len(inputs) == 0 {
		return nil
	}
	seen := make(map[Status]struct{}, len(inputs))
	var out []string
	add := func(s Status) {
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, string(s))
	}

	for _, raw := range inputs {
		s := strings.ToLower(strings.TrimSpace(raw))
		if s == "" {
			continue
		}
		// Canonical statuses first — fastest path, no map lookup.
		if Status(s).IsValid() {
			add(Status(s))
			continue
		}
		// Synonym expansion.
		if expanded, ok := StatusSynonyms[s]; ok {
			for _, e := range expanded {
				add(e)
			}
			continue
		}
		// Unknown — silently dropped. Callers that want strict
		// validation should check IsValid + StatusSynonyms manually.
	}
	return out
}

// SynonymKeys returns the synonym words sorted for stable output. Used
// by the prompt builder / docs to list what's accepted without
// hard-coding the list in two places.
func SynonymKeys() []string {
	keys := make([]string, 0, len(StatusSynonyms))
	for k := range StatusSynonyms {
		keys = append(keys, k)
	}
	// Sort for determinism — prompts read more cleanly in alphabetical
	// order, and tests can assert on the exact string.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

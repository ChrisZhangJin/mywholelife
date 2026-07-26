package store

import (
	"fmt"
	"strings"
)

// long-term-memory.md is a structured list, one line per long-term memory:
// "- <name> | <hook> | <memId>" under a stable header (D-06). memId is last and
// is `|`/newline-free by validKey construction, so it is the unambiguous key.
const indexHeader = "# Long-term memory index"

const hookMaxLen = 120

type indexLine struct {
	name  string
	hook  string
	memID string
}

// parseIndex reads the structured index into an ordered slice plus a
// memId→position map. Non-list lines (header, blank, placeholder) are ignored;
// duplicate memIds keep the first occurrence.
func parseIndex(content []byte) ([]indexLine, map[string]int) {
	var lines []indexLine
	idx := map[string]int{}
	for _, raw := range strings.Split(string(content), "\n") {
		s := strings.TrimSpace(raw)
		if !strings.HasPrefix(s, "- ") {
			continue
		}
		parts := strings.SplitN(s[2:], "|", 3)
		if len(parts) != 3 {
			continue
		}
		memID := strings.TrimSpace(parts[2])
		if memID == "" {
			continue
		}
		if _, ok := idx[memID]; ok {
			continue
		}
		idx[memID] = len(lines)
		lines = append(lines, indexLine{
			name:  strings.TrimSpace(parts[0]),
			hook:  strings.TrimSpace(parts[1]),
			memID: memID,
		})
	}
	return lines, idx
}

func serializeIndex(lines []indexLine) []byte {
	var b strings.Builder
	b.WriteString(indexHeader)
	b.WriteByte('\n')
	for _, l := range lines {
		b.WriteString("- ")
		b.WriteString(l.name)
		b.WriteString(" | ")
		b.WriteString(l.hook)
		b.WriteString(" | ")
		b.WriteString(l.memID)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// sanitizeField collapses '|', '\r', '\n' to spaces and trims — keeps a derived
// name/hook on a single parseable line (Pitfall 3).
func sanitizeField(s string) string {
	return strings.TrimSpace(strings.NewReplacer("|", " ", "\r", " ", "\n", " ").Replace(s))
}

// memName derives a human-friendly name from a memId by dropping a leading
// YYYYMMDD- date prefix (project keys); other keys pass through verbatim.
func memName(memID string) string {
	if len(memID) > 9 && memID[8] == '-' && allDigits(memID[:8]) {
		return memID[9:]
	}
	return memID
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// upsertIndexLine replaces m's line in place if present, else appends it, then
// re-serializes deterministically (idempotent: re-upserting yields identical
// bytes, D-07).
func upsertIndexLine(content []byte, m Memory) []byte {
	lines, idx := parseIndex(content)
	nl := indexLine{
		name:  sanitizeField(memName(m.MemID)),
		hook:  capLen(sanitizeField(m.Brief), hookMaxLen),
		memID: m.MemID,
	}
	if i, ok := idx[m.MemID]; ok {
		lines[i] = nl
	} else {
		lines = append(lines, nl)
	}
	return serializeIndex(lines)
}

// removeIndexLine drops memID's line. Removing an absent memId returns the input
// unchanged (byte-identical no-op).
func removeIndexLine(content []byte, memID string) []byte {
	lines, idx := parseIndex(content)
	i, ok := idx[memID]
	if !ok {
		return content
	}
	lines = append(lines[:i], lines[i+1:]...)
	return serializeIndex(lines)
}

// validateIndex asserts a 1:1 mapping between indexed rows and index lines
// (D-06): every long_term/tombstone memory has a line and every line has a
// long_term/tombstone row. Tombstones keep their line as the remind-able
// breadcrumb, so they are in the want set alongside long_term rows.
func validateIndex(rows []Memory, content []byte) error {
	_, idx := parseIndex(content)
	want := map[string]bool{}
	for _, m := range rows {
		if m.State != StateLongTerm && m.State != StateTombstone {
			continue
		}
		want[m.MemID] = true
		if _, ok := idx[m.MemID]; !ok {
			return fmt.Errorf("store: index missing line for %s memory %q", m.State, m.MemID)
		}
	}
	for memID := range idx {
		if !want[memID] {
			return fmt.Errorf("store: index has line for %q with no long_term/tombstone row", memID)
		}
	}
	return nil
}

func capLen(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

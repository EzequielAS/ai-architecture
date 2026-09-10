package scan

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"

	"quality-gate/internal/tsparser"
)

type tokenizedFile struct {
	path  string
	toks  []string // token values, comments and whitespace already dropped
	lines []int    // source line of each token
}

// occurrence is one token window: file index + index of its first token.
type occurrence struct{ file, tok int }

// diagonal identifies a constant offset between two files' token streams; a
// copy/paste block is a run of consecutive windows on the same diagonal.
type diagonal struct{ a, b, offset int }

// Duplication reports copy/paste blocks of at least minTokens tokens spanning at
// least minLines lines. Each pair of clones is reported once, on both sides.
func Duplication(files []string, minTokens, minLines int) []Finding {
	tokenized := make([]tokenizedFile, 0, len(files))
	for _, path := range files {
		if tf, ok := tokenize(path); ok && len(tf.toks) >= minTokens {
			tokenized = append(tokenized, tf)
		}
	}

	// Group the windows that hash alike, then merge them into maximal runs. Every
	// copy is paired with the first occurrence of the window rather than with all
	// the others: a block repeated N times costs N pairs instead of N², and each
	// copy still gets reported against that first occurrence.
	seeds := map[diagonal][][2]int{}
	for _, occs := range indexWindows(tokenized, minTokens) {
		if len(occs) < 2 {
			continue
		}
		first := occs[0]
		for _, other := range occs[1:] {
			key := diagonal{a: first.file, b: other.file, offset: other.tok - first.tok}
			seeds[key] = append(seeds[key], [2]int{first.tok, first.tok + minTokens})
		}
	}

	var findings []Finding
	for key, windows := range seeds {
		a, b := tokenized[key.a], tokenized[key.b]
		for _, run := range mergeRuns(windows) {
			start, end := run[0], run[1]
			// Within a single file, an offset shorter than the run itself means
			// the two ranges overlap — the same code, not a copy of it.
			if key.a == key.b && key.offset < end-start {
				continue
			}
			if !tokensEqual(a.toks, b.toks, start, end, key.offset) {
				continue // hash collision
			}
			if !varied(a.toks[start:end]) {
				continue
			}
			aFrom, aTo := a.lines[start], a.lines[end-1]
			bFrom, bTo := b.lines[start+key.offset], b.lines[end-1+key.offset]
			if aTo-aFrom+1 < minLines {
				continue
			}
			findings = append(findings,
				clone(a.path, aFrom, aTo, b.path, bFrom, bTo),
				clone(b.path, bFrom, bTo, a.path, aFrom, aTo))
		}
	}
	return dedupe(findings)
}

// minDistinctTokens is the token variety a block must have to count as a clone.
// A run of near-identical tokens — a literal table, a long repetition — matches
// itself at every offset and says nothing about copy/paste.
const minDistinctTokens = 8

func varied(window []string) bool {
	distinct := make(map[string]bool, minDistinctTokens)
	for _, tok := range window {
		distinct[tok] = true
		if len(distinct) >= minDistinctTokens {
			return true
		}
	}
	return false
}

func clone(file string, from, to int, otherFile string, otherFrom, otherTo int) Finding {
	return Finding{
		File: file, Line: from, Rule: "duplication",
		Message: fmt.Sprintf("linhas %d-%d duplicadas de %s:%d-%d — extraia o trecho comum",
			from, to, otherFile, otherFrom, otherTo),
	}
}

// indexWindows maps the hash of every minTokens-long window to its occurrences.
func indexWindows(files []tokenizedFile, minTokens int) map[uint64][]occurrence {
	index := map[uint64][]occurrence{}
	for fi, tf := range files {
		for i := 0; i+minTokens <= len(tf.toks); i++ {
			h := windowHash(tf.toks[i : i+minTokens])
			index[h] = append(index[h], occurrence{file: fi, tok: i})
		}
	}
	return index
}

func windowHash(window []string) uint64 {
	h := fnv.New64a()
	for _, v := range window {
		h.Write([]byte(v))
		h.Write([]byte{0}) // separator, so ["ab","c"] != ["a","bc"]
	}
	return h.Sum64()
}

func tokensEqual(a, b []string, start, end, offset int) bool {
	for i := start; i < end; i++ {
		j := i + offset
		if j < 0 || j >= len(b) || a[i] != b[j] {
			return false
		}
	}
	return true
}

// mergeRuns merges overlapping or contiguous [start,end) windows into runs, so a
// long clone is reported once instead of window by window.
func mergeRuns(windows [][2]int) [][2]int {
	if len(windows) == 0 {
		return nil
	}
	sort.Slice(windows, func(i, j int) bool { return windows[i][0] < windows[j][0] })
	merged := [][2]int{windows[0]}
	for _, w := range windows[1:] {
		last := &merged[len(merged)-1]
		if w[0] <= last[1] {
			if w[1] > last[1] {
				last[1] = w[1]
			}
			continue
		}
		merged = append(merged, w)
	}
	return merged
}

// dedupe keeps one finding per file+line, preferring the longest block.
func dedupe(findings []Finding) []Finding {
	best := map[string]Finding{}
	var order []string
	for _, f := range findings {
		key := fmt.Sprintf("%s:%d", f.File, f.Line)
		if prev, ok := best[key]; ok {
			if len(f.Message) <= len(prev.Message) {
				continue
			}
		} else {
			order = append(order, key)
		}
		best[key] = f
	}
	out := make([]Finding, 0, len(order))
	for _, key := range order {
		out = append(out, best[key])
	}
	return out
}

func tokenize(path string) (tokenizedFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tokenizedFile{}, false
	}
	tf := tokenizedFile{path: path}
	lex := tsparser.NewLexer(string(data))
	for {
		t := lex.Next()
		if t.Type == tsparser.TEOF {
			break
		}
		tf.toks = append(tf.toks, t.Value)
		tf.lines = append(tf.lines, t.Line)
	}
	return tf, true
}

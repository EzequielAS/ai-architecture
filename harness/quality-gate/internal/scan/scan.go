// Package scan implements the quality-gate checks: file size, code duplication
// and useEffect anti-patterns.
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Extensions analysed by every check.
var extensions = map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true}

// Finding is one violation reported by a check.
type Finding struct {
	File    string
	Line    int
	Rule    string
	Message string
}

// Ignore matches paths that must be skipped, from the --ignore globs.
type Ignore struct{ patterns []*regexp.Regexp }

// NewIgnore compiles glob patterns: `**` spans directories, `*` stays inside one
// path segment and `?` matches a single character.
func NewIgnore(globs []string) *Ignore {
	ig := &Ignore{}
	for _, g := range globs {
		if g == "" {
			continue
		}
		if re, err := regexp.Compile(globToRegexp(g)); err == nil {
			ig.patterns = append(ig.patterns, re)
		}
	}
	return ig
}

// Match reports whether path is ignored. A glob without a slash is also matched
// against the bare file name, so `--ignore '*.test.ts'` works at any depth.
func (ig *Ignore) Match(path string) bool {
	norm := normalize(path)
	base := filepath.Base(norm)
	for _, re := range ig.patterns {
		if re.MatchString(norm) || re.MatchString(base) {
			return true
		}
	}
	return false
}

func globToRegexp(glob string) string {
	var b strings.Builder
	b.WriteString("^")
	runes := []rune(glob)
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; c {
		case '*':
			if i+1 < len(runes) && runes[i+1] == '*' {
				i++
				if i+1 < len(runes) && runes[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?") // **/ also matches zero directories
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteRune('\\')
			b.WriteRune(c)
		default:
			b.WriteRune(c)
		}
	}
	b.WriteString("$")
	return b.String()
}

// Collect resolves the CLI paths into the list of files to analyse. Directories
// are walked recursively; `node_modules` and dot-directories are always skipped.
func Collect(paths []string, ig *Ignore) []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = normalize(p)
		if seen[p] || ig.Match(p) || !extensions[filepath.Ext(p)] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}

	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "aviso: %s ignorado (%v)\n", root, err)
			continue
		}
		if !info.IsDir() {
			add(root)
			continue
		}
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == "node_modules" || (name != "." && name != ".." && strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				if ig.Match(path) {
					return filepath.SkipDir
				}
				return nil
			}
			add(path)
			return nil
		})
	}
	sort.Strings(out)
	return out
}

func normalize(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(p), "./")
}

// Print writes the findings of one check grouped by file and returns the total.
func Print(title string, findings []Finding) int {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	fmt.Printf("\n=== %s ===\n\n", title)
	if len(findings) == 0 {
		fmt.Println("Nenhuma violação.")
	}

	current := ""
	for _, f := range findings {
		if f.File != current {
			if current != "" {
				fmt.Println()
			}
			fmt.Printf("%s\n", f.File)
			current = f.File
		}
		fmt.Printf("  [%s] line %d: %s\n", f.Rule, f.Line, f.Message)
	}

	fmt.Printf("\n--- %d violação(ões) ---\n", len(findings))
	return len(findings)
}

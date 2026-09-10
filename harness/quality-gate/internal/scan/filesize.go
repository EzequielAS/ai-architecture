package scan

import (
	"fmt"
	"os"
	"strings"
)

// FileSize reports every file longer than maxLines.
func FileSize(files []string, maxLines int) []Finding {
	var findings []Finding
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if n := countLines(data); n > maxLines {
			findings = append(findings, Finding{
				File: file, Line: 1, Rule: "file-too-large",
				Message: fmt.Sprintf("%d linhas (limite %d) — quebre o arquivo em módulos menores", n, maxLines),
			})
		}
	}
	return findings
}

// countLines counts source lines, ignoring a single trailing newline.
func countLines(data []byte) int {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

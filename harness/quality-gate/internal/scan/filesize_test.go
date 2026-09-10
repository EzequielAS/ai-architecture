package scan

import (
	"strings"
	"testing"
)

func TestFileSizeFlagsOversizedFile(t *testing.T) {
	files := writeFiles(t, map[string]string{
		"big.ts":   strings.Repeat("const x = 1;\n", 12),
		"small.ts": strings.Repeat("const x = 1;\n", 3),
	})

	findings := FileSize(files, 10)
	if len(findings) != 1 {
		t.Fatalf("esperava 1 finding, obteve %d: %v", len(findings), findings)
	}
	if !strings.HasSuffix(findings[0].File, "big.ts") {
		t.Errorf("arquivo errado: %s", findings[0].File)
	}
	if findings[0].Rule != "file-too-large" || findings[0].Line != 1 {
		t.Errorf("finding inesperado: %+v", findings[0])
	}
	if !strings.Contains(findings[0].Message, "12 linhas") {
		t.Errorf("mensagem sem a contagem real: %q", findings[0].Message)
	}
}

// The limit is exclusive: exactly maxLines lines is fine.
func TestFileSizeBoundary(t *testing.T) {
	files := writeFiles(t, map[string]string{"exact.ts": strings.Repeat("const x = 1;\n", 10)})

	if got := FileSize(files, 10); len(got) != 0 {
		t.Errorf("10 linhas com limite 10 não deveria violar: %v", got)
	}
	if got := FileSize(files, 9); len(got) != 1 {
		t.Errorf("10 linhas com limite 9 deveria violar, obteve %v", got)
	}
}

// A trailing newline must not inflate the count.
func TestFileSizeCountLines(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"", 0},
		{"a", 1},
		{"a\n", 1},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"a\n\nb\n", 3},
	}
	for _, tc := range cases {
		if got := countLines([]byte(tc.src)); got != tc.want {
			t.Errorf("countLines(%q) = %d, esperava %d", tc.src, got, tc.want)
		}
	}
}

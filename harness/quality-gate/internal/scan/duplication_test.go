package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFiles materialises a temp dir and returns the paths, sorted by name.
func writeFiles(t *testing.T, files map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return Collect([]string{dir}, NewIgnore(nil))
}

// block is a chunk of ~40 tokens over 6 lines.
const block = `export function calculateTotals(order) {
  const subtotal = order.items.reduce((sum, item) => sum + item.price * item.quantity, 0);
  const discount = subtotal > 100 ? subtotal * 0.1 : 0;
  const shipping = order.express ? 25 : 10;
  const tax = (subtotal - discount) * 0.07;
  return { subtotal: subtotal, discount: discount, shipping: shipping, tax: tax };
}
`

func TestDuplicationFindsCloneAcrossFiles(t *testing.T) {
	files := writeFiles(t, map[string]string{
		"a.ts": block,
		"b.ts": block,
	})

	findings := Duplication(files, 30, 5)
	if len(findings) != 2 {
		t.Fatalf("esperava 2 findings (um por lado), obteve %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Rule != "duplication" {
			t.Errorf("regra inesperada %q", f.Rule)
		}
		if !strings.Contains(f.Message, "duplicadas de") {
			t.Errorf("mensagem sem referência ao outro arquivo: %q", f.Message)
		}
	}
	// Each side must point at the other file.
	if strings.Contains(findings[0].Message, filepath.Base(findings[0].File)) {
		t.Errorf("finding aponta para si mesmo: %v", findings[0])
	}
}

func TestDuplicationRespectsMinTokens(t *testing.T) {
	files := writeFiles(t, map[string]string{"a.ts": block, "b.ts": block})

	if got := Duplication(files, 5000, 5); len(got) != 0 {
		t.Errorf("min-tokens alto deveria zerar os findings, obteve %v", got)
	}
}

func TestDuplicationRespectsMinLines(t *testing.T) {
	files := writeFiles(t, map[string]string{"a.ts": block, "b.ts": block})

	if got := Duplication(files, 30, 5); len(got) == 0 {
		t.Fatal("bloco de 6 linhas deveria ser reportado com min-lines=5")
	}
	if got := Duplication(files, 30, 50); len(got) != 0 {
		t.Errorf("min-lines alto deveria zerar os findings, obteve %v", got)
	}
}

// A long single-line clone must be filtered out by min-lines, not by min-tokens.
func TestDuplicationIgnoresSingleLineClone(t *testing.T) {
	line := "const config = { a: 1, b: 2, c: 3, d: 4, e: 5, f: 6, g: 7, h: 8, i: 9, j: 10 };\n"
	files := writeFiles(t, map[string]string{"a.ts": line, "b.ts": line})

	if got := Duplication(files, 20, 5); len(got) != 0 {
		t.Errorf("clone de uma linha não deveria passar por min-lines=5: %v", got)
	}
}

func TestDuplicationNoClones(t *testing.T) {
	files := writeFiles(t, map[string]string{
		"a.ts": "export const a = 1;\nexport const b = 2;\n",
		"b.ts": "export function totallyDifferent(x) {\n  return x * 3;\n}\n",
	})

	if got := Duplication(files, 30, 5); len(got) != 0 {
		t.Errorf("esperava nenhum finding, obteve %v", got)
	}
}

// A clone inside one file is reported, but overlapping token ranges are not.
func TestDuplicationWithinSingleFile(t *testing.T) {
	files := writeFiles(t, map[string]string{"a.ts": block + "\n" + strings.Replace(block, "calculateTotals", "calculateTotals2", 1)})

	if got := Duplication(files, 30, 5); len(got) == 0 {
		t.Error("esperava detectar copy/paste dentro do mesmo arquivo")
	}
}

// A long run of near-identical tokens matches itself at every offset; it must
// not turn into a flood of self-clone findings.
func TestDuplicationIgnoresRepeatedTokenRuns(t *testing.T) {
	files := writeFiles(t, map[string]string{"a.ts": strings.Repeat("a;\n", 2000)})

	if got := Duplication(files, 30, 5); len(got) != 0 {
		t.Errorf("esperava nenhum finding para tokens repetidos, obteve %d: %v", len(got), got[:min(3, len(got))])
	}
}

// A literal table repeated verbatim has too little token variety to be a
// meaningful copy/paste finding.
func TestDuplicationIgnoresLowVarietyBlocks(t *testing.T) {
	table := "export const flags = [\n" + strings.Repeat("  true,\n", 60) + "];\n"
	files := writeFiles(t, map[string]string{"a.ts": table, "b.ts": table})

	if got := Duplication(files, 30, 5); len(got) != 0 {
		t.Errorf("esperava nenhum finding para bloco pouco variado, obteve %v", got)
	}
}

// Every copy of a block must be reported, however many there are.
func TestDuplicationReportsEveryCopy(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("f%02d.ts", i)] = block
	}
	paths := writeFiles(t, files)

	findings := Duplication(paths, 30, 5)
	if len(findings) != 40 {
		t.Fatalf("esperava 1 finding por arquivo (40), obteve %d", len(findings))
	}
	reported := map[string]bool{}
	for _, f := range findings {
		reported[filepath.Base(f.File)] = true
	}
	if len(reported) != 40 {
		t.Errorf("esperava 40 arquivos distintos, obteve %d", len(reported))
	}
}

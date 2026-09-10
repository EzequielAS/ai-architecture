// quality-gate is a single-binary quality gate for TS/JS codebases. It runs
// three checks — file size, code duplication and useEffect anti-patterns — and
// exits non-zero when any of them reports a violation.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"quality-gate/internal/scan"
)

const (
	checkFileSize    = "file-size"
	checkDuplication = "duplication"
	checkEffects     = "effects"
)

var allChecks = []string{checkFileSize, checkDuplication, checkEffects}

// listFlag collects a repeatable flag that also accepts a comma-separated list.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }

func (l *listFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

func main() {
	fs := flag.NewFlagSet("quality-gate", flag.ExitOnError)
	fs.Usage = usage

	fileSize := fs.Int("file-size", 500, "limite de linhas por arquivo")
	dupTokens := fs.Int("dup-tokens", 50, "mínimo de tokens para considerar duplicação")
	dupLines := fs.Int("dup-lines", 5, "mínimo de linhas para considerar duplicação")

	var ignore listFlag
	fs.Var(&ignore, "ignore", "glob a ignorar (repetível ou separado por vírgula)")
	var checks listFlag
	fs.Var(&checks, "check", "checagem a rodar: file-size, duplication ou effects (padrão: todas)")
	var skips listFlag
	fs.Var(&skips, "skip", "checagem a pular (repetível ou separado por vírgula)")

	fs.Parse(os.Args[1:])

	selected, err := resolveChecks(checks, skips)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(2)
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	files := scan.Collect(paths, scan.NewIgnore(ignore))
	fmt.Printf("quality-gate — %d arquivo(s) analisado(s)\n", len(files))

	total := 0
	if selected[checkFileSize] {
		total += scan.Print(fmt.Sprintf("FILE SIZE (limite: %d linhas)", *fileSize),
			scan.FileSize(files, *fileSize))
	}
	if selected[checkDuplication] {
		total += scan.Print(fmt.Sprintf("DUPLICATION (mín.: %d tokens, %d linhas)", *dupTokens, *dupLines),
			scan.Duplication(files, *dupTokens, *dupLines))
	}
	if selected[checkEffects] {
		total += scan.Print("USEEFFECT", scan.Effects(files))
	}

	fmt.Printf("\n=== TOTAL: %d violação(ões) ===\n", total)
	if total > 0 {
		os.Exit(1)
	}
}

// resolveChecks combines --check and --skip into the set of checks to run.
// Without --check every check is selected; --skip then removes from that set.
func resolveChecks(requested, skipped []string) (map[string]bool, error) {
	selected := map[string]bool{}
	if len(requested) == 0 {
		for _, c := range allChecks {
			selected[c] = true
		}
	} else {
		for _, c := range requested {
			if err := validCheck(c, "--check"); err != nil {
				return nil, err
			}
			selected[c] = true
		}
	}
	for _, c := range skipped {
		if err := validCheck(c, "--skip"); err != nil {
			return nil, err
		}
		delete(selected, c)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("nenhuma checagem restou para rodar — --skip removeu todas")
	}
	return selected, nil
}

// validCheck rejects a name that is not one of allChecks.
func validCheck(name, flagName string) error {
	for _, c := range allChecks {
		if c == name {
			return nil
		}
	}
	names := append([]string{}, allChecks...)
	sort.Strings(names)
	return fmt.Errorf("checagem desconhecida %q em %s — use uma de: %s", name, flagName, strings.Join(names, ", "))
}

func usage() {
	fmt.Fprint(os.Stderr, `quality-gate — quality gate para TS/JS

Uso:
  quality-gate [flags] [caminho ...]

Caminhos podem ser arquivos ou diretórios (padrão: "."). Apenas .ts, .tsx, .js e
.jsx são analisados; node_modules e diretórios ocultos são sempre ignorados.
Sai com código 1 quando há violações.

Flags:
  --file-size N     limite de linhas por arquivo (padrão 500)
  --dup-tokens N    mínimo de tokens duplicados (padrão 50)
  --dup-lines N     mínimo de linhas duplicadas (padrão 5)
  --ignore GLOB     glob a ignorar; repetível ou separado por vírgula
  --check NOME      roda só a checagem indicada: file-size, duplication, effects;
                    repetível (padrão: todas)
  --skip NOME       pula a checagem indicada; repetível ou separado por vírgula

Exemplos:
  quality-gate --file-size 300 --dup-tokens 60 --ignore '**/*.test.*' src
  quality-gate --skip effects src
  quality-gate --skip effects,duplication src
`)
}

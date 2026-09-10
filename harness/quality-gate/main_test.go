package main

import (
	"sort"
	"strings"
	"testing"
)

func names(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestResolveChecks(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		skipped   []string
		want      []string
		wantErr   string
	}{
		{
			name: "sem flags roda todas",
			want: []string{checkDuplication, checkEffects, checkFileSize},
		},
		{
			name:      "check restringe",
			requested: []string{checkEffects},
			want:      []string{checkEffects},
		},
		{
			name:    "skip remove das padrão",
			skipped: []string{checkEffects},
			want:    []string{checkDuplication, checkFileSize},
		},
		{
			name:    "skip múltiplo",
			skipped: []string{checkEffects, checkDuplication},
			want:    []string{checkFileSize},
		},
		{
			name:      "skip aplicado depois do check",
			requested: []string{checkEffects, checkFileSize},
			skipped:   []string{checkEffects},
			want:      []string{checkFileSize},
		},
		{
			name:    "skip de checagem não selecionada é inócuo",
			skipped: []string{checkEffects, checkEffects},
			want:    []string{checkDuplication, checkFileSize},
		},
		{
			name:    "skip de todas é erro",
			skipped: allChecks,
			wantErr: "nenhuma checagem restou",
		},
		{
			name:    "skip desconhecido é erro",
			skipped: []string{"efeitos"},
			wantErr: `checagem desconhecida "efeitos" em --skip`,
		},
		{
			name:      "check desconhecido é erro",
			requested: []string{"tamanho"},
			wantErr:   `checagem desconhecida "tamanho" em --check`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveChecks(tt.requested, tt.skipped)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("erro = %v, queria conter %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if strings.Join(names(got), ",") != strings.Join(tt.want, ",") {
				t.Errorf("checagens = %v, queria %v", names(got), tt.want)
			}
		})
	}
}

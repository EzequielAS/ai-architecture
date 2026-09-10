package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree builds a temp directory and returns its root.
func tree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("const x = 1;\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// relative strips the temp root so assertions stay readable.
func relative(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func TestCollectFiltersExtensionsAndVendorDirs(t *testing.T) {
	root := tree(t,
		"src/a.ts", "src/b.tsx", "src/c.js", "src/d.jsx",
		"src/e.css", "src/f.json", "README.md",
		"node_modules/pkg/index.js",
		".next/build.js",
		"src/nested/deep/g.ts",
	)

	got := relative(root, Collect([]string{root}, NewIgnore(nil)))
	want := []string{"src/a.ts", "src/b.tsx", "src/c.js", "src/d.jsx", "src/nested/deep/g.ts"}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestCollectAcceptsExplicitFiles(t *testing.T) {
	root := tree(t, "src/a.ts", "src/b.ts")

	got := relative(root, Collect([]string{filepath.Join(root, "src/a.ts")}, NewIgnore(nil)))
	if len(got) != 1 || got[0] != "src/a.ts" {
		t.Errorf("esperava apenas src/a.ts, obteve %v", got)
	}
}

func TestCollectDedupes(t *testing.T) {
	root := tree(t, "src/a.ts")
	file := filepath.Join(root, "src/a.ts")

	if got := Collect([]string{file, file, root}, NewIgnore(nil)); len(got) != 1 {
		t.Errorf("esperava 1 arquivo, obteve %v", got)
	}
}

func TestIgnoreGlobs(t *testing.T) {
	root := tree(t,
		"src/a.ts",
		"src/a.test.ts",
		"src/types.d.ts",
		"src/generated/api.ts",
		"src/mocks/handlers.ts",
		"dist/bundle.js",
	)

	cases := []struct {
		name  string
		globs []string
		want  []string
	}{
		{
			name:  "no globs",
			globs: nil,
			want:  []string{"dist/bundle.js", "src/a.test.ts", "src/a.ts", "src/generated/api.ts", "src/mocks/handlers.ts", "src/types.d.ts"},
		},
		{
			name:  "test files anywhere",
			globs: []string{"**/*.test.*"},
			want:  []string{"dist/bundle.js", "src/a.ts", "src/generated/api.ts", "src/mocks/handlers.ts", "src/types.d.ts"},
		},
		{
			name:  "declaration files",
			globs: []string{"**/*.d.ts"},
			want:  []string{"dist/bundle.js", "src/a.test.ts", "src/a.ts", "src/generated/api.ts", "src/mocks/handlers.ts"},
		},
		{
			name:  "whole directory",
			globs: []string{"**/generated/**", "**/mocks/**"},
			want:  []string{"dist/bundle.js", "src/a.test.ts", "src/a.ts", "src/types.d.ts"},
		},
		{
			name:  "bare filename glob",
			globs: []string{"*.test.ts"},
			want:  []string{"dist/bundle.js", "src/a.ts", "src/generated/api.ts", "src/mocks/handlers.ts", "src/types.d.ts"},
		},
		{
			name:  "several globs at once",
			globs: []string{"**/*.test.*", "**/*.d.ts", "**/generated/**"},
			want:  []string{"dist/bundle.js", "src/a.ts", "src/mocks/handlers.ts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := relative(root, Collect([]string{root}, NewIgnore(tc.globs)))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got  %v\nwant %v", got, tc.want)
			}
		})
	}
}

// Ignoring a directory must prune the walk, relative to the given path too.
func TestIgnoreRelativeDirectory(t *testing.T) {
	root := tree(t, "src/a.ts", "src/skip/b.ts")
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	got := Collect([]string{"."}, NewIgnore([]string{"src/skip/**"}))
	if len(got) != 1 || got[0] != "src/a.ts" {
		t.Errorf("esperava [src/a.ts], obteve %v", got)
	}
}

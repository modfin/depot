package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	err := os.MkdirAll(filepath.Dir(p), 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(p, nil, 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func writeGitignore(t *testing.T, root, rel, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func relFiles(t *testing.T, root string, files []string) []string {
	t.Helper()
	var out []string
	for _, f := range files {
		r, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(r))
	}
	slices.Sort(out)
	return out
}

func TestFindDepFilesRespectsGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod")
	writeFile(t, root, "pnpm-lock.yaml")
	writeFile(t, root, "ignored/go.mod")
	writeFile(t, root, "sub/package-lock.json")
	writeFile(t, root, "sub/cargo.lock")
	writeFile(t, root, "sub/nested/pom.xml")
	writeGitignore(t, root, ".gitignore", "ignored/\n*.lock\npnpm-lock.yaml\n")
	writeGitignore(t, root, "sub/.gitignore", "nested/\n")

	got := relFiles(t, root, findDepFiles(root, true, nil, false))
	want := []string{"go.mod", "sub/package-lock.json"}
	if !slices.Equal(got, want) {
		t.Errorf("respecting gitignore: got %v, want %v", got, want)
	}

	got = relFiles(t, root, findDepFiles(root, true, nil, true))
	want = []string{"go.mod", "ignored/go.mod", "pnpm-lock.yaml", "sub/cargo.lock", "sub/nested/pom.xml", "sub/package-lock.json"}
	if !slices.Equal(got, want) {
		t.Errorf("no-gitignore: got %v, want %v", got, want)
	}
}

func TestFindDepFilesGitignoreNonRecurse(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "go.mod")
	writeFile(t, root, "pnpm-lock.yaml")
	writeGitignore(t, root, ".gitignore", "pnpm-lock.yaml\n")

	got := relFiles(t, root, findDepFiles(root, false, nil, false))
	if want := []string{"go.mod"}; !slices.Equal(got, want) {
		t.Errorf("respecting gitignore: got %v, want %v", got, want)
	}

	got = relFiles(t, root, findDepFiles(root, false, nil, true))
	if want := []string{"go.mod", "pnpm-lock.yaml"}; !slices.Equal(got, want) {
		t.Errorf("no-gitignore: got %v, want %v", got, want)
	}
}

func TestFindDepFilesGitignoreNegation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "dist/keep/go.mod")
	writeFile(t, root, "dist/other/go.mod")
	writeGitignore(t, root, ".gitignore", "dist/*\n!dist/keep\n")

	got := relFiles(t, root, findDepFiles(root, true, nil, false))
	if want := []string{"dist/keep/go.mod"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

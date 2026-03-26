package pnpm

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParsePackageKey(t *testing.T) {
	tests := []struct {
		key     string
		name    string
		version string
	}{
		// v9: no leading slash, separator is @
		{"elasticlunr@0.9.5", "elasticlunr", "0.9.5"},
		// v9: scoped package
		{"@11ty/eleventy@3.1.5", "@11ty/eleventy", "3.1.5"},
		// v9: peer dep suffix must be stripped
		{"zod-validation-error@5.0.0(zod@4.3.6)", "zod-validation-error", "5.0.0"},
		// v9: nested peer dep suffix
		{"@scope/pkg@1.2.3(react@18.3.1(react-dom@18.3.1))", "@scope/pkg", "1.2.3"},

		// v6: leading slash, separator is @
		{"/accepts@1.3.8", "accepts", "1.3.8"},
		// v6: scoped package with leading slash
		{"/@adobe/css-tools@4.4.4", "@adobe/css-tools", "4.4.4"},

		// v5: leading slash, separator is /
		{"/clsx/1.2.1", "clsx", "1.2.1"},
		// v5: scoped package
		{"/@babel/generator/7.17.7", "@babel/generator", "7.17.7"},

		// Malformed: no version separator
		{"no-version", "no-version", ""},
		{"@scoped/no-version", "@scoped/no-version", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			name, version := ParsePackageKey(tt.key)
			if name != tt.name {
				t.Errorf("ParsePackageKey(%q): name = %q, want %q", tt.key, name, tt.name)
			}
			if version != tt.version {
				t.Errorf("ParsePackageKey(%q): version = %q, want %q", tt.key, version, tt.version)
			}
		})
	}
}

func TestParseV5Lockfile(t *testing.T) {
	b, err := os.ReadFile("v5.4-pnpm-lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var lf LockFile
	if err = yaml.Unmarshal(b, &lf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// v5 has top-level dependencies and devDependencies maps
	if _, ok := lf.Dependencies["clsx"]; !ok {
		t.Error("expected 'clsx' in top-level dependencies")
	}
	if _, ok := lf.DevDependencies["typescript"]; !ok {
		t.Error("expected 'typescript' in top-level devDependencies")
	}
	// Importers should be empty for v5
	if len(lf.Importers) != 0 {
		t.Errorf("expected no importers in v5, got %d", len(lf.Importers))
	}

	// Dev packages should have Dev: true
	entry, ok := lf.Packages["/@babel/generator/7.17.7"]
	if !ok {
		t.Fatal("expected package '/@babel/generator/7.17.7'")
	}
	if !entry.Dev {
		t.Error("expected /@babel/generator/7.17.7 to have dev: true")
	}
}

func TestParseV6Lockfile(t *testing.T) {
	b, err := os.ReadFile("v6-pnpm-lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var lf LockFile
	if err = yaml.Unmarshal(b, &lf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// v6 has top-level dependencies with specifier/version objects
	if _, ok := lf.Dependencies["express"]; !ok {
		t.Error("expected 'express' in top-level dependencies")
	}
	// No importers in v6
	if len(lf.Importers) != 0 {
		t.Errorf("expected no importers in v6, got %d", len(lf.Importers))
	}

	// Prod packages should have Dev: false
	entry, ok := lf.Packages["/accepts@1.3.8"]
	if !ok {
		t.Fatal("expected package '/accepts@1.3.8'")
	}
	if entry.Dev {
		t.Error("expected /accepts@1.3.8 to have dev: false")
	}

	// express package should be present
	if _, ok := lf.Packages["/express@4.19.2"]; !ok {
		t.Error("expected package '/express@4.19.2'")
	}
}

func TestParseV9Lockfile(t *testing.T) {
	b, err := os.ReadFile("v9-pnpm-lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var lf LockFile
	if err = yaml.Unmarshal(b, &lf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// v9 uses importers
	root, ok := lf.Importers["."]
	if !ok {
		t.Fatal("expected importers['.']")
	}

	// Check direct prod dep
	dep, ok := root.Dependencies["elasticlunr"]
	if !ok {
		t.Fatal("expected elasticlunr in importers['.'].dependencies")
	}
	if dep.Version != "0.9.5" {
		t.Errorf("elasticlunr version = %q, want %q", dep.Version, "0.9.5")
	}

	// Check direct dev dep
	if _, ok := root.DevDependencies["@11ty/eleventy"]; !ok {
		t.Error("expected @11ty/eleventy in importers['.'].devDependencies")
	}

	// Packages section should use clean name@version keys (no leading slash)
	if _, ok := lf.Packages["elasticlunr@0.9.5"]; !ok {
		t.Error("expected package 'elasticlunr@0.9.5' (v9 format, no leading slash)")
	}
	if _, ok := lf.Packages["@11ty/eleventy@3.1.5"]; !ok {
		t.Error("expected package '@11ty/eleventy@3.1.5' (v9 scoped format)")
	}
}

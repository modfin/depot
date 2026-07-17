package gitignore

import "testing"

func TestPatternMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		isDir   bool
		want    bool
	}{
		// basename match at any depth
		{"foo", "foo", false, true},
		{"foo", "a/foo", false, true},
		{"foo", "a/b/foo", false, true},
		{"foo", "bar/foo", true, true},
		{"foo", "foo/bar", false, true}, // ancestor dir match
		{"foo", "foobar", false, false},
		{"foo", "a/foobar", false, false},

		// anchored
		{"/foo", "foo", false, true},
		{"/foo", "a/foo", false, false},
		{"a/b", "a/b", false, true},
		{"a/b", "a/b/c", false, true},
		{"a/b", "x/a/b", false, false},
		{"doc/frotz/", "doc/frotz", true, true},
		{"doc/frotz/", "a/doc/frotz", true, false},

		// dir-only
		{"build/", "build", true, true},
		{"build/", "build", false, false},
		{"build/", "build/x", false, true},
		{"build/", "a/build", true, true},
		{"build/", "a/build/x.txt", false, true},

		// globs
		{"*.log", "err.log", false, true},
		{"*.log", "a/err.log", false, true},
		{"*.log", "err.log.txt", false, false},
		{"?.txt", "a.txt", false, true},
		{"?.txt", "ab.txt", false, false},
		{"[abc].txt", "b.txt", false, true},
		{"[abc].txt", "d.txt", false, false},
		{"[!abc].txt", "d.txt", false, true},
		{"[!abc].txt", "a.txt", false, false},
		{"*.min.js", "a/b/x.min.js", false, true},

		// '*' does not cross '/'
		{"a/*/c", "a/b/c", false, true},
		{"a/*/c", "a/b/b2/c", false, false},
		{"a/*", "a", true, false},
		{"a/*", "a/b", true, true},
		{"a/*", "a/b/c", false, true},

		// **
		{"**/foo", "foo", false, true},
		{"**/foo", "a/b/foo", false, true},
		{"foo/**", "foo", true, false}, // contents only, not the dir itself
		{"foo/**", "foo/x", false, true},
		{"foo/**", "foo/x/y", false, true},
		{"a/**/b", "a/b", false, true},
		{"a/**/b", "a/x/b", false, true},
		{"a/**/b", "a/x/y/b", false, true},
		{"a/**/b", "a/b/x", false, true}, // a/b matched as ancestor
		{"a/**/b", "a/xb", false, false},
		{"**", "x", false, true},
		{"**", "x/y", false, true},
	}

	for _, tt := range tests {
		p, ok := ParsePattern(tt.pattern)
		if !ok {
			t.Errorf("ParsePattern(%q) not ok", tt.pattern)
			continue
		}
		if got := p.Match(tt.path, tt.isDir); got != tt.want {
			t.Errorf("pattern %q path %q isDir %v: got %v, want %v", tt.pattern, tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestParsePatternSkips(t *testing.T) {
	for _, line := range []string{"", "   ", "# comment", "!", "/", "!/", "\r"} {
		if _, ok := ParsePattern(line); ok {
			t.Errorf("ParsePattern(%q) should not produce a pattern", line)
		}
	}
}

func TestParsePatternTrimming(t *testing.T) {
	p, ok := ParsePattern("foo   \r")
	if !ok {
		t.Fatal("expected pattern")
	}
	if !p.Match("foo", false) {
		t.Error("trailing spaces and CR should be trimmed")
	}
}

func TestParse(t *testing.T) {
	ps := Parse("# comment\n\nfoo\r\n!bar\nbaz/\n")
	if len(ps) != 3 {
		t.Fatalf("got %d patterns, want 3", len(ps))
	}
}

func TestMatcherLastMatchWins(t *testing.T) {
	var m Matcher
	m.Append("", Parse("foo\n!foo\n"))
	if m.Ignored("foo", false) {
		t.Error("negation after match should unignore")
	}

	var m2 Matcher
	m2.Append("", Parse("!foo\nfoo\n"))
	if !m2.Ignored("foo", false) {
		t.Error("match after negation should ignore")
	}
}

func TestMatcherNestedOverride(t *testing.T) {
	var m Matcher
	m.Append("", Parse("*.lock\n"))
	m.Append("sub", Parse("!important.lock\n"))

	if !m.Ignored("other.lock", false) {
		t.Error("root *.lock should ignore other.lock")
	}
	if !m.Ignored("sub/other.lock", false) {
		t.Error("root *.lock should ignore sub/other.lock")
	}
	if m.Ignored("sub/important.lock", false) {
		t.Error("deeper negation should override root pattern")
	}
}

func TestMatcherScopeIsolation(t *testing.T) {
	var m Matcher
	m.Append("a", Parse("secret\n"))

	if !m.Ignored("a/secret", false) {
		t.Error("a/secret should be ignored")
	}
	if !m.Ignored("a/x/secret", false) {
		t.Error("a/x/secret should be ignored")
	}
	if m.Ignored("b/secret", false) {
		t.Error("scope a must not apply to b")
	}
	if m.Ignored("secret", false) {
		t.Error("scope a must not apply to root")
	}
}

func TestMatcherUnignoreDirIdiom(t *testing.T) {
	var m Matcher
	m.Append("", Parse("dist/*\n!dist/keep\n"))

	if m.Ignored("dist", true) {
		t.Error("dist/* must not match dist itself")
	}
	if !m.Ignored("dist/other", true) {
		t.Error("dist/other should be ignored")
	}
	if m.Ignored("dist/keep", true) {
		t.Error("dist/keep should be re-included")
	}
	if m.Ignored("dist/keep/go.mod", false) {
		t.Error("dist/keep/go.mod should be re-included")
	}
}

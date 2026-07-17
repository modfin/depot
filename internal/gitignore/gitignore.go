// Package gitignore implements gitignore(5) pattern matching for the
// common pattern set: comments, negation, dir-only, anchoring, '*',
// '?', '**' and character classes. Backslash escapes are not supported.
package gitignore

import (
	"path"
	"strings"
)

type Pattern struct {
	negate   bool
	dirOnly  bool
	segments []string
}

// ParsePattern parses one gitignore line. ok is false for blanks and comments.
func ParsePattern(line string) (p Pattern, ok bool) {
	line = strings.TrimRight(line, " \r")
	if line == "" || strings.HasPrefix(line, "#") {
		return Pattern{}, false
	}
	if strings.HasPrefix(line, "!") {
		p.negate = true
		line = line[1:]
	}
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimRight(line, "/")
	}
	anchored := strings.HasPrefix(line, "/")
	line = strings.TrimPrefix(line, "/")
	if line == "" {
		return Pattern{}, false
	}
	anchored = anchored || strings.Contains(line, "/")

	// path.Match negates classes with '^', fnmatch with '!'
	line = strings.ReplaceAll(line, "[!", "[^")

	p.segments = strings.Split(line, "/")
	if !anchored {
		// a pattern without '/' matches at any depth
		p.segments = append([]string{"**"}, p.segments...)
	}
	return p, true
}

// Parse parses .gitignore content.
func Parse(content string) []Pattern {
	var ps []Pattern
	for _, line := range strings.Split(content, "\n") {
		if p, ok := ParsePattern(line); ok {
			ps = append(ps, p)
		}
	}
	return ps
}

// Match reports whether the pattern matches rel, a slash-separated path
// relative to the directory holding the .gitignore file.
func (p Pattern) Match(rel string, isDir bool) bool {
	exact, prefix := matchSegments(p.segments, strings.Split(rel, "/"))
	if prefix {
		// matched an ancestor directory, everything under it is matched
		return true
	}
	return exact && (!p.dirOnly || isDir)
}

// matchSegments matches pattern segments against path segments.
// exact: pattern consumed the whole path.
// prefix: pattern matched a strict ancestor of the path.
func matchSegments(pat, segs []string) (exact, prefix bool) {
	if len(pat) == 0 {
		return len(segs) == 0, len(segs) > 0
	}
	if pat[0] == "**" {
		if len(pat) == 1 {
			// trailing ** matches contents, not the dir itself
			return len(segs) > 0, len(segs) > 1
		}
		for i := 0; i <= len(segs); i++ {
			e, pre := matchSegments(pat[1:], segs[i:])
			exact = exact || e
			prefix = prefix || pre
			if exact && prefix {
				break
			}
		}
		return exact, prefix
	}
	if len(segs) == 0 {
		return false, false
	}
	if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
		return false, false
	}
	return matchSegments(pat[1:], segs[1:])
}

type scope struct {
	base     string
	patterns []Pattern
}

// Matcher evaluates patterns from .gitignore files in multiple
// directories. Add scopes parents-first so deeper files take precedence.
type Matcher struct {
	scopes []scope
}

// Append adds patterns from a .gitignore located in base, a
// slash-separated directory path relative to the root ("" for the root).
func (m *Matcher) Append(base string, patterns []Pattern) {
	if len(patterns) == 0 {
		return
	}
	m.scopes = append(m.scopes, scope{base: base, patterns: patterns})
}

// Ignored reports whether rel, slash-separated and relative to the
// root, is excluded. The last matching pattern wins.
func (m *Matcher) Ignored(rel string, isDir bool) bool {
	var ignored bool
	for _, s := range m.scopes {
		sub, ok := inScope(s.base, rel)
		if !ok {
			continue
		}
		for _, p := range s.patterns {
			if p.Match(sub, isDir) {
				ignored = !p.negate
			}
		}
	}
	return ignored
}

func inScope(base, rel string) (string, bool) {
	if base == "" {
		return rel, true
	}
	if !strings.HasPrefix(rel, base+"/") {
		return "", false
	}
	return rel[len(base)+1:], true
}

package pnpm

import "strings"

type LockFile struct {
	LockfileVersion interface{} `yaml:"lockfileVersion"`
	// v5/v6: top-level dependency maps (value is string for v5, object for v6)
	Dependencies    map[string]interface{} `yaml:"dependencies"`
	DevDependencies map[string]interface{} `yaml:"devDependencies"`
	// v9: per-workspace importers
	Importers map[string]Importer `yaml:"importers"`
	// All versions: resolved packages (clean name@version keys in v9, /name@version in v6, /name/version in v5)
	Packages map[string]PackageEntry `yaml:"packages"`
}

// Importer is a v9 workspace entry containing direct dependency declarations.
type Importer struct {
	Dependencies    map[string]DepEntry `yaml:"dependencies"`
	DevDependencies map[string]DepEntry `yaml:"devDependencies"`
}

// DepEntry is used by v6/v9 format where each dep has a specifier and resolved version.
type DepEntry struct {
	Specifier string `yaml:"specifier"`
	Version   string `yaml:"version"`
}

// PackageEntry holds per-package metadata. The Dev field is only set in v5/v6.
type PackageEntry struct {
	Dev      bool `yaml:"dev"`
	Optional bool `yaml:"optional"`
}

// ParsePackageKey extracts the package name and version from a pnpm lockfile package key.
// It handles all three lockfile formats:
//   - v9: "name@version" or "@scope/name@version"
//   - v6: "/name@version" or "/@scope/name@version"
//   - v5: "/name/version" or "/@scope/name/version"
//
// Peer dependency suffixes (e.g. "(react@18.3.1)") are stripped from the version.
func ParsePackageKey(key string) (name, version string) {
	// Strip leading / used in v5/v6 formats.
	key = strings.TrimPrefix(key, "/")

	// Find the last @ that separates the package name from its version.
	// We require lastAt > 0 so that a scoped-package leading @ (e.g. @scope/name)
	// is not mistakenly treated as the version separator.
	// Search only in the portion before any peer-dep suffix "(..." so that an @
	// inside the suffix (e.g. "name@1.0.0(peer@2.0.0)") is not used as the split point.
	searchKey := key
	if parenIdx := strings.Index(key, "("); parenIdx >= 0 {
		searchKey = key[:parenIdx]
	}
	lastAt := strings.LastIndex(searchKey, "@")
	if lastAt > 0 {
		name = searchKey[:lastAt]
		version = searchKey[lastAt+1:]
		return name, version
	}

	// No usable @ found → v5 format where separator is /.
	if strings.HasPrefix(key, "@") {
		// Scoped v5: @scope/name/version
		// Skip past "@scope/" to find the second slash.
		firstSlash := strings.Index(key, "/")
		if firstSlash < 0 {
			return key, ""
		}
		rest := key[firstSlash+1:] // "name/version"
		nextSlash := strings.Index(rest, "/")
		if nextSlash < 0 {
			return key, ""
		}
		name = key[:firstSlash+1+nextSlash] // "@scope/name"
		version = rest[nextSlash+1:]        // "version"
		return name, version
	}

	// Non-scoped v5: name/version
	lastSlash := strings.LastIndex(key, "/")
	if lastSlash < 0 {
		return key, ""
	}
	return key[:lastSlash], key[lastSlash+1:]
}

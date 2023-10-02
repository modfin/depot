package depsdev

type Version struct {
	VersionKey struct {
		System  string `json:"system"`
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"versionKey"`
	IsDefault    bool     `json:"isDefault"`
	Licenses     []string `json:"licenses"`
	AdvisoryKeys []any    `json:"advisoryKeys"`
	Links        []struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	} `json:"links"`
	SlsaProvenances []any `json:"slsaProvenances"`
}

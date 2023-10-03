package cargo

type Pkg struct {
	Name         string   `toml:"name"`
	Version      string   `toml:"version"`
	Source       string   `toml:"source,omitempty"`
	Checksum     string   `toml:"checksum,omitempty"`
	Dependencies []string `toml:"dependencies,omitempty"`
}
type Lockfile struct {
	Packages []Pkg `toml:"package"`
}

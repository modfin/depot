package deps

import (
	"depot"
	"depot/internal/depsdev"
	"encoding/json"
	"fmt"
	"github.com/aquasecurity/go-dep-parser/pkg/nodejs/npm"
	"github.com/modfin/henry/exp/containerz/set"
	"github.com/modfin/henry/mapz"
	"golang.org/x/mod/modfile"
	"os"
	"path/filepath"
	"strings"
)

func ToLicense(dir string, deps []Dep) depot.License {
	root := depot.License{}
	for _, d := range deps {

		name := fmt.Sprintf("%s %s", d.Name, d.Version)
		if d.Indirect {
			name = name + " //indirect"
		}
		for _, l := range d.License {
			ll := root[depot.SPDX(l)]
			if ll == nil {
				ll = map[depot.FileName][]string{}
				root[depot.SPDX(l)] = ll
			}

			context, err := filepath.Rel(dir, d.Context)
			if err != nil {
				context = "!" + d.Context
			}
			ll[depot.FileName(context)] = append(ll[depot.FileName(context)], name)

		}

	}

	return root
}

type Dep struct {
	Context  string
	Type     depsdev.DepType
	Name     string
	Version  string
	Indirect bool
	License  []string
}

func FromFile(path string) ([]Dep, error) {
	filename := filepath.Base(path)

	switch strings.ToLower(filename) {
	case "package-lock.json":
		return From(path, depsdev.NPM)
	case "go.mod":
		return From(path, depsdev.GO)
	case "pom.xml":
		return From(path, depsdev.MAVEN)
	case "cargo.toml":
		return From(path, depsdev.CARGO)
	}
	return nil, fmt.Errorf("could not find any dep type associated with file name %s", filename)

}

func From(file string, _type depsdev.DepType) ([]Dep, error) {
	switch _type {
	case depsdev.GO:
		return FromGO(file)
	case depsdev.NPM:
		return FromNPM(file)
	}

	return nil, fmt.Errorf("type %s does not exist", _type)

}

func FromNPM(path string) (deps []Dep, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lockfile npm.LockFile
	err = json.Unmarshal(b, &lockfile)
	if err != nil {
		return nil, err
	}

	direct := set.From(mapz.Keys(lockfile.Packages[""].Dependencies)...)

	for name, d := range lockfile.Dependencies {
		// Ignore dev deps
		if d.Dev {
			continue
		}

		l, _ := depsdev.New().Licenses(depsdev.NPM, name, d.Version)
		if len(l) == 0 {
			l = []string{"UNKNOWN"}
		}

		deps = append(deps, Dep{
			Context:  path,
			Type:     depsdev.NPM,
			Name:     name,
			Version:  d.Version,
			Indirect: !direct.Exists(name),
			License:  l,
		})
	}
	return deps, nil
}

func FromGO(path string) (deps []Dep, err error) {

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, err := modfile.Parse("go.mod", b, nil)
	if err != nil {
		return nil, err
	}

	for _, r := range file.Require {
		l, _ := depsdev.New().Licenses(depsdev.GO, r.Mod.Path, r.Mod.Version)
		if len(l) == 0 {
			l = []string{"UNKNOWN"}
		}
		deps = append(deps, Dep{
			Context:  path,
			Type:     depsdev.GO,
			Name:     r.Mod.Path,
			Version:  r.Mod.Version,
			Indirect: r.Indirect,
			License:  l,
		})
	}
	return deps, nil
}

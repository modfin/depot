package deps

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/modfin/depot"
	"github.com/modfin/depot/internal/deps/cargo"
	"github.com/modfin/depot/internal/deps/npm"
	"github.com/modfin/depot/internal/deps/pom"
	"github.com/modfin/depot/internal/depsdev"
	"github.com/modfin/henry/exp/containerz/set"
	"github.com/modfin/henry/mapz"
	"github.com/modfin/henry/slicez"
	log "github.com/sirupsen/logrus"
	"golang.org/x/mod/modfile"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func New(cache *Cache) *Processor {
	return &Processor{
		cache: cache,
	}
}

type Processor struct {
	cache *Cache
}

func ToLicense(rootdir string, deps []Dep) depot.LicenseStructure {
	root := depot.LicenseStructure{}
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

			context, err := filepath.Rel(rootdir, d.Context)
			if err != nil {
				context = "!" + d.Context
			}
			ll[depot.FileName(context)] = append(ll[depot.FileName(context)], name)
		}
	}
	return root
}

type Dep struct {
	Context  string          `json:"-"`
	Type     depsdev.DepType `json:"t"`
	Name     string          `json:"n"`
	Version  string          `json:"v"`
	Indirect bool            `json:"-"`
	License  []string        `json:"l"`
}

func (d Dep) Key() string {
	return DepKey(d.Type, d.Name, d.Version)
}
func DepKey(_type depsdev.DepType, name string, version string) string {
	return fmt.Sprintf("%s|%s|%s", _type, name, version)
}

func (pro *Processor) FromFile(path string) ([]Dep, error) {
	filename := filepath.Base(path)

	switch strings.ToLower(filename) {
	case "package-lock.json":
		return pro.From(path, depsdev.NPM)
	case "go.mod":
		return pro.From(path, depsdev.GO)
	case "pom.xml":
		return pro.From(path, depsdev.MAVEN)
	case "cargo.lock":
		return pro.From(path, depsdev.CARGO)
	case "requirements.txt":
		return pro.From(path, depsdev.PYPI)
	}

	return nil, fmt.Errorf("could not find any dep type associated with file name %s", filename)

}

func (pro *Processor) From(file string, _type depsdev.DepType) ([]Dep, error) {
	switch _type {
	case depsdev.GO:
		return pro.FromGO(file)
	case depsdev.NPM:
		return pro.FromNPM(file)
	case depsdev.MAVEN:
		return pro.FromMaven(file)
	case depsdev.CARGO:
		return pro.FromCargo(file)
	case depsdev.PYPI:
		return pro.FromPypi(file)
	}

	return nil, fmt.Errorf("type %s does not exist", _type)

}

func (pro *Processor) FromNPM(path string) (deps []Dep, err error) {
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

	seen := map[string]string{}

	for _, name := range mapz.Keys(lockfile.Packages) {
		canonical := slicez.Nth(strings.Split(name, "node_modules/"), -1)
		if seen[canonical] == "" {
			seen[canonical] = name
		}
		if len(name) < len(seen[canonical]) {
			seen[canonical] = name
		}
	}

	merged := mapz.Merge(mapz.Remap(seen, func(canonical, name string) (string, npm.Dependency) {
		v := lockfile.Packages[name]
		return canonical, npm.Dependency{
			Version: v.Version,
			Dev:     v.Dev,
			//Dependencies: v.Dependencies,
			//Resolved:     v.Resolved,
			//StartLine:    v.StartLine,
			//EndLine:      v.EndLine,
		}
	}), lockfile.Dependencies)

	for name, d := range merged {
		if name == "" { // self...
			continue
		}
		// Ignore dev deps
		if d.Dev {
			continue
		}
		name = slicez.Nth(strings.Split(name, "node_modules/"), -1)

		l, _ := pro.LicensesOf(depsdev.NPM, name, d.Version)

		deps = append(deps, Dep{
			Context:  path,
			Type:     depsdev.NPM,
			Name:     name,
			Version:  d.Version,
			Indirect: !direct.Exists(name),
			License:  l,
		})
	}
	return slicez.UniqBy(deps, func(a Dep) string {
		return a.Key()
	}), nil
}

func (pro *Processor) FromCargo(lockFilePath string) (deps []Dep, err error) {

	b, err := os.ReadFile(lockFilePath)
	if err != nil {
		return nil, err
	}
	var lockfile cargo.Lockfile
	err = toml.Unmarshal(b, &lockfile)
	if err != nil {
		return nil, err
	}

	p, _ := slicez.Find(lockfile.Packages, func(pkg cargo.Pkg) bool {
		return pkg.Source == ""
	})
	direct := set.From(p.Dependencies...)

	for _, d := range lockfile.Packages {

		if d.Name == p.Name {
			continue
		}

		l, _ := pro.LicensesOf(depsdev.CARGO, d.Name, d.Version)

		deps = append(deps, Dep{
			Context:  lockFilePath,
			Type:     depsdev.CARGO,
			Name:     d.Name,
			Version:  d.Version,
			Indirect: !direct.Exists(d.Name),
			License:  l,
		})
	}
	return deps, nil
}

func (pro *Processor) FromGO(path string) (deps []Dep, err error) {

	//TODO recurese down indirect deps if wanted.

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	file, err := modfile.Parse("go.mod", b, nil)
	if err != nil {
		return nil, err
	}

	for _, r := range file.Require {
		l, _ := pro.LicensesOf(depsdev.GO, r.Mod.Path, r.Mod.Version)

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

func (pro *Processor) FromMaven(path string) (deps []Dep, err error) {

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var x pom.PomXML
	err = xml.Unmarshal(b, &x)
	if err != nil {
		return nil, err
	}

	p := pom.POM{Content: &x}

	props := p.Properties()

	for _, d := range p.Content.Dependencies.Dependency {

		d := d.Resolve(props, p.Content.DependencyManagement.Dependencies.Dependency, nil)

		// Ignore test deps
		if d.Scope == "test" {
			continue
		}

		name := fmt.Sprintf("%s:%s", d.GroupID, d.ArtifactID)

		l, _ := pro.LicensesOf(depsdev.MAVEN, name, d.Version)

		deps = append(deps, Dep{
			Context:  path,
			Type:     depsdev.MAVEN,
			Name:     name,
			Version:  d.Version,
			Indirect: false,
			License:  l,
		})
	}
	return deps, nil
}
func (pro *Processor) FromPypi(path string) (deps []Dep, err error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)

	rStripByKey := func(line string, key string) string {
		if pos := strings.Index(line, key); pos >= 0 {
			line = strings.TrimRightFunc((line)[:pos], unicode.IsSpace)
		}
		return line
	}

	removeExtras := func(line string) string {
		startIndex := strings.Index(line, "[")
		endIndex := strings.Index(line, "]") + 1
		if startIndex != -1 && endIndex != -1 {
			line = line[:startIndex] + line[endIndex:]
		}
		return line
	}

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.ReplaceAll(line, " ", "")
		line = strings.ReplaceAll(line, `\`, "")
		line = removeExtras(line)
		line = rStripByKey(line, "#")
		line = rStripByKey(line, ";")
		line = rStripByKey(line, "--")
		s := strings.Split(line, "==")
		if len(s) != 2 {
			continue
		}

		name := s[0]
		version := s[1]

		licence, _ := pro.LicensesOf(depsdev.PYPI, name, version)
		deps = append(deps, Dep{
			Context:  path,
			Type:     depsdev.PYPI,
			Name:     name,
			Version:  version,
			Indirect: false,
			License:  licence,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return deps, nil
}

func (pro *Processor) LicensesOf(depType depsdev.DepType, name string, version string) ([]string, error) {
	key := DepKey(depType, name, version)

	if pro.cache != nil {
		dep, found := pro.cache.Get(key)

		if found {
			log.Infof("deps.dev; licence cache hit for %s", dep.Key())
			return dep.License, nil
		}
	}

	log.Infof("deps.dev; requesting %s", DepKey(depType, name, version))
	v, err := depsdev.New().Version(depType, name, version)

	if err != nil && err.Error() == "http status 404" {
		err = nil
	}
	if err != nil {
		log.WithError(err).Fatalf("could not retrive dep %s %s %s", depType, name, version)
	}

	license := slicez.Map(v.Licenses, func(a string) string {
		if a == "non-standard" {
			a = "~non-standard"
		}
		return a
	})

	if len(license) == 0 {
		license = []string{"~unknown"}
	}

	if pro.cache != nil {
		pro.cache.Put(Dep{
			Type:    depType,
			Name:    name,
			Version: version,
			License: license,
		})
	}

	return license, err
}

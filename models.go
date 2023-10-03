package depot

import (
	"fmt"
	"github.com/modfin/henry/mapz"
	"github.com/modfin/henry/slicez"
	"strings"
)

type SPDX string
type FileName string

type LicenseStructure map[SPDX]map[FileName][]string

func (license LicenseStructure) String() string {

	var header string
	var body string

	spdxs := slicez.Sort(mapz.Keys(license))

	for _, spdx := range spdxs {

		var link string

		if !slicez.Contains([]string{"~unknown", "~non-standard"}, string(spdx)) {

			ss := strings.ReplaceAll(string(spdx), "(", "( ")
			ss = strings.ReplaceAll(ss, ")", " )")
			links := slicez.Map(strings.Split(ss, " "), func(a string) string {
				switch a {
				case "AND", "OR":
					return "\n " + a
				case "WITH":
					return "\n  " + a
				case "(", ")":
					return a
				}
				return fmt.Sprintf("https://spdx.org/licenses/%s.html", a)
			})

			link = fmt.Sprintf("\n %s", strings.Join(links, " "))
		}

		body += fmt.Sprintf("========================================================================\n")
		body += fmt.Sprintf("%s%s\n", spdx, link)
		body += fmt.Sprintf("========================================================================\n\n")

		var countDirect int
		var countIndirect int
		files := slicez.Sort(mapz.Keys(license[spdx]))
		for _, file := range files {
			body += fmt.Sprintf(" [%s]\n", file)

			deps := slicez.Sort(license[spdx][file])
			for _, dep := range deps {
				if strings.HasSuffix(dep, " //indirect") {
					continue
				}
				countDirect++
				body += fmt.Sprintf("   %s\n", dep)
			}
			for _, dep := range deps {
				if !strings.HasSuffix(dep, " //indirect") {
					continue
				}
				countIndirect++
				body += fmt.Sprintf("   %s\n", dep)
			}
			body += "\n"
		}

		header += fmt.Sprintf("%s: %d\n", spdx, countDirect+countIndirect)

	}
	return "---\n" + header + "---\n" + body
}

type Config struct {
	Dependency struct {
		Ignore   []Dependency `yaml:"ignore"`
		Licenses []Dependency `yaml:"licenses"`
	} `yaml:"dependency"`
}

type Dependency struct {
	Type    string   `yaml:"type"`
	Name    string   `yaml:"name"`
	Version string   `yaml:"version"`
	License []string `yaml:"license"`
}

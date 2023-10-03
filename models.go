package depot

import (
	"depot/internal/depsdev"
	"fmt"
	"github.com/modfin/henry/mapz"
	"github.com/modfin/henry/slicez"
	"strings"
)

type SPDX string
type FileName string

type License map[SPDX]map[FileName][]string

func (license License) String() string {

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
				case "AND", "OR", "WITH", ")", "(":
					return a
				}

				return fmt.Sprintf("https://spdx.org/licenses/%s.html", a)
			})

			link = fmt.Sprintf("  //  %s", strings.Join(links, " "))
		}

		body += fmt.Sprintf("[%s]%s\n\n", spdx, link)

		var countDirect int
		var countIndirect int
		files := slicez.Sort(mapz.Keys(license[spdx]))
		for _, file := range files {
			body += fmt.Sprintf(" [[%s]]\n", file)

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

		body += "\n"
	}
	return "---\n" + header + "---\n" + body
}

type LicenseCache struct {
	Type    depsdev.DepType
	Name    string
	Version string
	License []string
}

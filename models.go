package depot

import (
	"fmt"
	"github.com/modfin/henry/mapz"
	"github.com/modfin/henry/slicez"
	"strings"
)

type SPDX string
type FileName string

type License map[SPDX]map[FileName][]string

func (license License) String() string {

	var agg string

	spdxs := slicez.Sort(mapz.Keys(license))
	for _, spdx := range spdxs {
		agg += fmt.Sprintf("[%s]\n", spdx)

		files := slicez.Sort(mapz.Keys(license[spdx]))
		for _, file := range files {
			agg += fmt.Sprintf(" [[%s]]\n", file)

			deps := slicez.Sort(license[spdx][file])
			for _, dep := range deps {
				if strings.HasSuffix(dep, " //indirect") {
					continue
				}
				agg += fmt.Sprintf("   %s\n", dep)
			}
			for _, dep := range deps {
				if !strings.HasSuffix(dep, " //indirect") {
					continue
				}
				agg += fmt.Sprintf("   %s\n", dep)
			}
		}
		agg += "\n"
	}
	return agg
}

package deps

import (
	"github.com/modfin/henry/slicez"
	"testing"
)

func TestMultiVersionedDependencyFromNPM(t *testing.T) {
	/*
		Processor.FromNPM utilizes slicez.Nth(strings.Split(name, "node_modules/"), -1)

		We'll get the same name for a dependency given the following case:
			name: node_modules/parse5/node_modules/entities -> [parse5/, entities]
			name: node_modules/entities -> [entities]

		We'll end up with a non-deterministic version if the 'entities'-dependency's versions differ,
		since the underlying datastructure is a map. Thus, Depot will fail intermittently.
	*/

	p := Processor{}
	for i := 0; i < 50; i++ {
		deps, err := p.FromNPM("./npm/multi-versioned-dep_package-lock.json")
		if err != nil {
			t.Fatal(err)
		}
		entitiesDep, found := slicez.Find(deps, func(dep Dep) bool {
			return dep.Name == "entities"
		})
		if !found {
			t.Fatalf("expected 'entities' dependency")
		}

		if entitiesDep.Version != "3.0.1" {
			t.Fatalf("expected 'entities' to have version '3.0.1', got %s", entitiesDep.Version)
		}
	}

}

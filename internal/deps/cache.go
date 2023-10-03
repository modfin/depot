package deps

import (
	"encoding/json"
	"github.com/modfin/henry/mapz"
	"github.com/modfin/henry/slicez"
	"os"
)

type Cache struct {
	file string
	c    map[string]Dep
}

func (c *Cache) Put(dep Dep) {
	c.c[dep.Key()] = dep
}

func (c *Cache) Get(key string) (Dep, bool) {
	v, ok := c.c[key]
	return v, ok
}

func (c *Cache) Save() error {
	deps := mapz.Values(c.c)
	deps = slicez.SortFunc(deps, func(a, b Dep) bool {
		return a.Key() < b.Key()
	})

	b, err := json.Marshal(deps)
	if err != nil {
		return err
	}
	return os.WriteFile(c.file, b, 0644)
}

func NewCache(file string) (*Cache, error) {
	var c Cache
	c.file = file
	c.c = map[string]Dep{}

	d, err := os.ReadFile(file)
	if err != nil {
		return &c, err
	}

	if len(d) == 0 {
		d = []byte("[]")
	}

	var deps []Dep
	err = json.Unmarshal(d, &deps)
	if err != nil {
		return &c, err
	}

	c.c = slicez.KeyBy(deps, func(a Dep) string {
		return a.Key()

	})

	return &c, nil
}

package depsdev

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type DepType string

const NPM DepType = "npm"
const GO DepType = "go"
const MAVEN DepType = "maven"
const CARGO DepType = "cargo"

const PYPI DepType = "pypi"

type Client struct {
	uri string
}

func New() *Client {
	return &Client{
		uri: "https://api.deps.dev/v3alpha",
	}
}

//https://docs.deps.dev/api/v3alpha/
//https://api.deps.dev/v3alpha/systems/npm/packages/jquery
//https://api.deps.dev/v3alpha/systems/npm/packages/jquery/versions/3.7.1
//https://api.deps.dev/v3alpha/systems/maven/packages/org.postgresql:postgresql
//https://api.deps.dev/v3alpha/systems/maven/packages/org.postgresql:postgresql/versions/42.3.8
//https://api.deps.dev/v3alpha/systems/go/packages/github.com%2Fmodfin%2Fhenry
//https://api.deps.dev/v3alpha/systems/go/packages/github.com%2Fmodfin%2Fhenry/versions/v0.0.0-20230824150253-35f12224ee68
//https://api.deps.dev/v3alpha/systems/go/packages/github.com%2Flabstack%2Fecho%2Fv4
//https://api.deps.dev/v3alpha/systems/go/packages/github.com%2Flabstack%2Fecho%2Fv4/versions/v4.9.1
//https://api.deps.dev/v3alpha/systems/cargo/packages/rand:packageversions
//https://api.deps.dev/v3alpha/systems/cargo/packages/rand/versions/0.8.5
//

func (c *Client) Version(depType DepType, name string, version string) (Version, error) {
	var v Version
	res, err := http.DefaultClient.Get(fmt.Sprintf("%s/systems/%s/packages/%s/versions/%s", c.uri, url.PathEscape(string(depType)), url.PathEscape(name), url.PathEscape(version)))
	if err != nil {
		return v, err
	}
	defer res.Body.Close()

	if res.StatusCode > 299 {
		return v, fmt.Errorf("http status %d", res.StatusCode)
	}

	err = json.NewDecoder(res.Body).Decode(&v)
	return v, err
}

func (c *Client) Licenses(depType DepType, name string, version string) ([]string, error) {
	v, err := c.Version(depType, name, version)
	return v.Licenses, err
}

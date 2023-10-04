package pom

import (
	"encoding/xml"
	"fmt"
	"github.com/modfin/henry/slicez"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"
)

type POM struct {
	Content *PomXML
}

func mergeMaps(parent, child map[string]string) map[string]string {
	if parent == nil {
		return child
	}
	for k, v := range child {
		parent[k] = v
	}
	return parent
}

func (p POM) Properties() Properties {
	props := p.Content.Properties
	return mergeMaps(props, p.ProjectProperties())
}

func (p POM) ProjectProperties() map[string]string {
	val := reflect.ValueOf(p.Content).Elem()
	props := p.ListProperties(val)

	// "version" and "groupId" elements could be inherited from parent.
	// https://maven.apache.org/pom.html#inheritance
	props["groupId"] = p.Content.GroupId
	props["version"] = p.Content.Version

	// https://maven.apache.org/pom.html#properties
	projectProperties := map[string]string{}
	for k, v := range props {
		if strings.HasPrefix(k, "project.") {
			continue
		}

		// e.g. ${project.groupId}
		key := fmt.Sprintf("project.%s", k)
		projectProperties[key] = v

		// It is deprecated, but still available.
		// e.g. ${groupId}
		projectProperties[k] = v
	}

	return projectProperties
}

func (p POM) ListProperties(val reflect.Value) map[string]string {
	props := map[string]string{}
	for i := 0; i < val.NumField(); i++ {
		f := val.Type().Field(i)

		tag, ok := f.Tag.Lookup("xml")
		if !ok || strings.Contains(tag, ",") {
			// e.g. ",chardata"
			continue
		}

		switch f.Type.Kind() {
		case reflect.Slice:
			continue
		case reflect.Map:
			m := val.Field(i)
			for _, e := range m.MapKeys() {
				v := m.MapIndex(e)
				props[e.String()] = v.String()
			}
		case reflect.Struct:
			nestedProps := p.ListProperties(val.Field(i))
			for k, v := range nestedProps {
				key := fmt.Sprintf("%s.%s", tag, k)
				props[key] = v
			}
		default:
			props[tag] = val.Field(i).String()
		}
	}
	return props
}

func (p POM) Licenses() []string {
	fliterd := slicez.Filter(p.Content.Licenses.License, func(a PomLicense) bool {
		return a.Name != ""
	})
	return slicez.Map(fliterd, func(a PomLicense) string {
		return a.Name
	})
}

func (p POM) Repositories() []string {
	var urls []string
	for _, rep := range p.Content.Repositories.Repository {
		if rep.Releases.Enabled != "false" {
			urls = append(urls, rep.URL)
		}
	}
	return urls
}

type PomXML struct {
	Parent     PomParent   `xml:"parent"`
	GroupId    string      `xml:"groupId"`
	ArtifactId string      `xml:"artifactId"`
	Version    string      `xml:"version"`
	Licenses   PomLicenses `xml:"licenses"`
	Modules    struct {
		Text   string   `xml:",chardata"`
		Module []string `xml:"module"`
	} `xml:"modules"`
	Properties           Properties `xml:"properties"`
	DependencyManagement struct {
		Text         string          `xml:",chardata"`
		Dependencies PomDependencies `xml:"dependencies"`
	} `xml:"dependencyManagement"`
	Dependencies PomDependencies `xml:"dependencies"`
	Repositories struct {
		Text       string `xml:",chardata"`
		Repository []struct {
			Text     string `xml:",chardata"`
			ID       string `xml:"id"`
			Name     string `xml:"name"`
			URL      string `xml:"url"`
			Releases struct {
				Text    string `xml:",chardata"`
				Enabled string `xml:"enabled"`
			} `xml:"releases"`
			Snapshots struct {
				Text    string `xml:",chardata"`
				Enabled string `xml:"enabled"`
			} `xml:"snapshots"`
		} `xml:"repository"`
	} `xml:"repositories"`
}

type PomParent struct {
	GroupId      string `xml:"groupId"`
	ArtifactId   string `xml:"artifactId"`
	Version      string `xml:"version"`
	RelativePath string `xml:"relativePath"`
}

type PomLicenses struct {
	Text    string       `xml:",chardata"`
	License []PomLicense `xml:"license"`
}

type PomLicense struct {
	Name string `xml:"name"`
}

type PomDependencies struct {
	Text       string          `xml:",chardata"`
	Dependency []PomDependency `xml:"dependency"`
}

type PomDependency struct {
	Text       string        `xml:",chardata"`
	GroupID    string        `xml:"groupId"`
	ArtifactID string        `xml:"artifactId"`
	Version    string        `xml:"version"`
	Scope      string        `xml:"scope"`
	Optional   bool          `xml:"optional"`
	Exclusions PomExclusions `xml:"exclusions"`
}

type PomExclusions struct {
	Text      string         `xml:",chardata"`
	Exclusion []PomExclusion `xml:"exclusion"`
}

// ref. https://maven.apache.org/guides/introduction/introduction-to-optional-and-excludes-dependencies.html
type PomExclusion struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
}

func (d PomDependency) Name() string {
	return fmt.Sprintf("%s:%s", d.GroupID, d.ArtifactID)
}

// Resolve evaluates variables in the dependency and inherit some fields from dependencyManagement to the dependency.
func (d PomDependency) Resolve(props map[string]string, depManagement, rootDepManagement []PomDependency) PomDependency {
	// Evaluate variables
	dep := PomDependency{
		Text:       d.Text,
		GroupID:    evaluateVariable(d.GroupID, props, nil),
		ArtifactID: evaluateVariable(d.ArtifactID, props, nil),
		Version:    evaluateVariable(d.Version, props, nil),
		Scope:      evaluateVariable(d.Scope, props, nil),
		Optional:   d.Optional,
		Exclusions: d.Exclusions,
	}

	// If this dependency is managed in the root POM,
	// we need to overwrite fields according to the managed dependency.
	if managed, found := findDep(d.Name(), rootDepManagement); found { // dependencyManagement from the root POM
		if managed.Version != "" {
			dep.Version = evaluateVariable(managed.Version, props, nil)
		}
		if managed.Scope != "" {
			dep.Scope = evaluateVariable(managed.Scope, props, nil)
		}
		if managed.Optional {
			dep.Optional = managed.Optional
		}
		if len(managed.Exclusions.Exclusion) != 0 {
			dep.Exclusions = managed.Exclusions
		}
		return dep
	}

	// Inherit version, scope and optional from dependencyManagement if empty
	if managed, found := findDep(d.Name(), depManagement); found { // dependencyManagement from parent
		if dep.Version == "" {
			dep.Version = evaluateVariable(managed.Version, props, nil)
		}
		if dep.Scope == "" {
			dep.Scope = evaluateVariable(managed.Scope, props, nil)
		}
		// TODO: need to check the behavior
		if !dep.Optional {
			dep.Optional = managed.Optional
		}
		if len(dep.Exclusions.Exclusion) == 0 {
			dep.Exclusions = managed.Exclusions
		}
	}
	return dep
}

type Properties map[string]string

type property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

func (props *Properties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	*props = Properties{}
	for {
		var p property
		err := d.Decode(&p)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		(*props)[p.XMLName.Local] = p.Value
	}
	return nil
}

func findDep(name string, depManagement []PomDependency) (PomDependency, bool) {
	return slicez.Find(depManagement, func(item PomDependency) bool {
		return item.Name() == name
	})
}

var varRegexp = regexp.MustCompile(`\${(\S+?)}`)

func evaluateVariable(s string, props map[string]string, seenProps []string) string {
	if props == nil {
		props = map[string]string{}
	}

	for _, m := range varRegexp.FindAllStringSubmatch(s, -1) {
		var newValue string

		// env.X: https://maven.apache.org/pom.html#Properties
		// e.g. env.PATH
		if strings.HasPrefix(m[1], "env.") {
			newValue = os.Getenv(strings.TrimPrefix(m[1], "env."))
		} else {
			// <Properties> might include another property.
			// e.g. <animal.sniffer.skip>${skipTests}</animal.sniffer.skip>
			ss, ok := props[m[1]]
			if ok {
				// search for looped Properties
				if slices.Contains(seenProps, ss) {
					printLoopedPropertiesStack(m[0], seenProps)
					return ""
				}
				seenProps = append(seenProps, ss) // save evaluated props to check if we get this prop again
				newValue = evaluateVariable(ss, props, seenProps)
				seenProps = []string{} // clear props if we returned from recursive. Required for correct work with 2 same props like ${foo}-${foo}
			}

		}
		s = strings.ReplaceAll(s, m[0], newValue)
	}
	return s
}
func printLoopedPropertiesStack(env string, usedProps []string) {
	var s string
	for _, prop := range usedProps {
		s += fmt.Sprintf("%s -> ", prop)
	}
	log.Warnf("Lopped Properties were detected: %s%s", s, env)
}

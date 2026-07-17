package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/modfin/depot"
	"github.com/modfin/depot/internal/deps"
	"github.com/modfin/depot/internal/depsdev"
	"github.com/modfin/depot/internal/gitignore"
	"github.com/modfin/henry/slicez"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func main() {

	log.SetLevel(log.ErrorLevel)

	var cache *deps.Cache
	var config depot.Config

	app := &cli.App{
		Name:  "depot",
		Usage: "a dep license tool",
		Flags: []cli.Flag{

			&cli.StringFlag{
				Name:  "cache-file",
				Value: ".depot.cache.json",
			},
			&cli.StringFlag{
				Name:  "license-file",
				Value: "LICENSES_DEP",
			},
			&cli.StringFlag{
				Name:  "config-file",
				Value: ".depot.yml",
			},

			&cli.StringFlag{
				Name:  "root",
				Value: ".",
			},
			&cli.BoolFlag{
				Name:    "recurse",
				Usage:   "Recurse down the directory structure searching for files of interest",
				Aliases: []string{"r"},
			},
			&cli.StringSliceFlag{
				Name:        "type",
				Usage:       "Type of dep files we are looking for, go, npm, pnpm, maven, cargo",
				DefaultText: "All",
				Aliases:     []string{"t"},
			},
			&cli.BoolFlag{
				Name:  "no-gitignore",
				Usage: "Do not respect .gitignore files when searching for dep files",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
			},
		},
		Before: func(c *cli.Context) error {
			if c.Bool("verbose") {
				log.SetLevel(log.InfoLevel)
			}

			root := c.String("root")
			cacheFile := filepath.Join(root, c.String("cache-file"))
			touch(cacheFile)

			var err error
			cache, err = deps.NewCache(cacheFile)
			if err != nil {
				return err
			}

			configFile := filepath.Join(root, c.String("config-file"))
			touch(configFile)
			b, err := os.ReadFile(configFile)
			if err != nil {
				return err
			}
			err = yaml.Unmarshal(b, &config)
			if err != nil {
				return err
			}
			return nil
		},
		After: func(context *cli.Context) error {
			return cache.Save()
		},

		Commands: []*cli.Command{
			{
				Name: "print",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "lint"},
				},
				Action: func(c *cli.Context) error {
					files := depFiles(c)

					p := deps.New(cache)

					var allDeps []deps.Dep
					for _, file := range files {
						d, err := p.FromFile(file)
						if err != nil {
							log.WithError(err).Error("could not resolve ", file)
							continue
						}
						allDeps = append(allDeps, d...)
					}
					allDeps = fixDeps(config, allDeps)

					l := deps.ToLicense(c.String("root"), allDeps)

					fmt.Println(l.String())

					if c.Bool("lint") {
						return lint(allDeps)
					}
					return nil
				},
			},
			{
				Name: "save",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "lint"},
				},
				Action: func(c *cli.Context) error {
					files := depFiles(c)

					p := deps.New(cache)

					var allDeps []deps.Dep
					for _, file := range files {
						d, err := p.FromFile(file)
						if err != nil {
							log.WithError(err).Error("could not resolve ", file)
							continue
						}
						allDeps = append(allDeps, d...)
					}
					allDeps = fixDeps(config, allDeps)

					l := deps.ToLicense(c.String("root"), allDeps)

					rootdir := c.String("root")
					outname := c.String("license-file")

					err := os.WriteFile(filepath.Join(rootdir, outname), []byte(l.String()), 0644)
					if err != nil {
						return err
					}

					if c.Bool("lint") {
						return lint(allDeps)
					}
					return nil
				},
			},
			{
				Name: "lint",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "verify", Usage: "Verify against existing --license-file, and fail on unsaved dependency or version changes"},
				},
				Action: func(c *cli.Context) error {
					files := depFiles(c)
					p := deps.New(cache)

					var allDeps []deps.Dep
					for _, file := range files {
						d, err := p.FromFile(file)
						if err != nil {
							log.WithError(err).Error("could not resolve ", file)
							continue
						}
						allDeps = append(allDeps, d...)
					}
					allDeps = fixDeps(config, allDeps)
					err := lint(allDeps)
					if err != nil {
						return err
					}
					if c.Bool("verify") {
						return verifyNewDeps(c, allDeps)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func lint(allDeps []deps.Dep) error {

	failingDeps := slicez.Filter(allDeps, func(d deps.Dep) bool {
		return slicez.ContainsFunc(d.License, func(e string) bool {
			return strings.HasPrefix(e, "~")
		})
	})
	failingDeps = slicez.SortFunc(failingDeps, func(a, b deps.Dep) bool {
		return a.Key() < b.Key()
	})

	if len(failingDeps) > 0 {
		log.Error("There are dependencies with unclear license, address them in .depot.yml")
		log.Error("Failing dependencies are:")
		for _, d := range failingDeps {
			log.Errorf("- %s %s %s", d.Type, d.Name, d.Version)
		}
		return errors.New("failed lint")
	}
	return nil
}

func verifyNewDeps(c *cli.Context, allDeps []deps.Dep) error {
	newLicenseStructure := deps.ToLicense(c.String("root"), allDeps).String()
	b, err := os.ReadFile(c.String("license-file"))
	if err != nil {
		log.WithError(err).Errorf("could not read existing license-file: '%s', run 'save' to generate file", c.String("license-file"))
		return err
	}
	existingLicenseStructure := string(b)
	if newLicenseStructure != existingLicenseStructure {
		log.Error("There are new changes to be persisted due to dependency changes:")
		diff := cmp.Diff(existingLicenseStructure, newLicenseStructure)
		fmt.Println(diff)
		err = errors.New("failed to verify against local source - run save command")
		return err
	}
	return nil
}

func fixDeps(config depot.Config, ds []deps.Dep) []deps.Dep {

	var n []deps.Dep

	for _, d := range ds {

		_, found := slicez.Find(config.Dependency.Ignore, func(e depot.Dependency) bool {
			return e.Type == string(d.Type) && e.Name == d.Name && (d.Version == e.Version || e.Version == "*" || e.Version == "")
		})

		if found {
			continue
		}

		match, found := slicez.Find(config.Dependency.Licenses, func(e depot.Dependency) bool {
			return e.Type == string(d.Type) && e.Name == d.Name && (d.Version == e.Version || e.Version == "*" || e.Version == "")
		})

		if found {
			d.License = match.License
		}
		n = append(n, d)

	}
	return n

}

func depFiles(c *cli.Context) []string {

	if c.Args().Len() > 0 {
		return c.Args().Slice()
	}

	return findDepFiles(c.String("root"), c.Bool("recurse"), c.StringSlice("type"), c.Bool("no-gitignore"))
}

func findDepFiles(root string, recurse bool, types []string, noGitignore bool) []string {
	var files []string

	var ignores *gitignore.Matcher
	if !noGitignore {
		ignores = &gitignore.Matcher{}
	}

	_ = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {

		base := filepath.Base(path)

		if path != root && !recurse && info.IsDir() {
			return filepath.SkipDir
		}

		if ignores != nil && path != root {
			rel, err := filepath.Rel(root, path)
			if err == nil && ignores.Ignored(filepath.ToSlash(rel), info.IsDir()) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				if depType(base) != "" {
					log.Info("gitignored, skipping ", path)
				}
				return nil
			}
		}

		if ignores != nil && info.IsDir() {
			loadGitignore(ignores, root, path)
		}

		// ignoring hidden files
		if path != root && strings.HasPrefix(base, ".") {
			return nil
		}

		// ignoring node_modules
		if path != root && info.IsDir() && base == "node_modules" {
			return filepath.SkipDir
		}

		t := depType(base)
		if t != "" {
			if len(types) == 0 || slicez.Contains(types, t) {
				files = append(files, path)
			}
		}

		return nil
	})
	return files

}

func depType(base string) string {
	switch strings.ToLower(base) {
	case "package-lock.json", "pnpm-lock.yaml":
		return string(depsdev.NPM)
	case "go.mod":
		return string(depsdev.GO)
	case "pom.xml":
		return string(depsdev.MAVEN)
	case "cargo.lock":
		return string(depsdev.CARGO)
	case "requirements.txt":
		return string(depsdev.PYPI)
	}
	return ""
}

func loadGitignore(m *gitignore.Matcher, root, dir string) {
	name := filepath.Join(dir, ".gitignore")
	b, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.WithError(err).Warn("could not read ", name)
		return
	}
	base, err := filepath.Rel(root, dir)
	if err != nil {
		return
	}
	if base == "." {
		base = ""
	}
	m.Append(filepath.ToSlash(base), gitignore.Parse(string(b)))
}

func touch(fileName string) {
	_, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		file, err := os.Create(fileName)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}
}

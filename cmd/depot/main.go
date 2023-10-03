package main

import (
	"fmt"
	"github.com/modfin/depot"
	"github.com/modfin/depot/internal/deps"
	"github.com/modfin/depot/internal/depsdev"
	"github.com/modfin/henry/slicez"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
				Usage:       "Type of dep files we are looking for, go, npm, maven, cargo",
				DefaultText: "All",
				Aliases:     []string{"t"},
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
						lint(allDeps)
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
						lint(allDeps)
					}
					return nil
				},
			},
			{
				Name: "lint",
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
					lint(allDeps)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func lint(allDeps []deps.Dep) {

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
		os.Exit(1)
	}
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

	return findDepFiles(c.String("root"), c.Bool("recurse"), c.StringSlice("type"))
}

func findDepFiles(root string, recurse bool, types []string) []string {
	var files []string

	_ = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {

		base := filepath.Base(path)

		if path != root && !recurse && info.IsDir() {
			return filepath.SkipDir
		}

		// ignoring hidden files
		if path != root && strings.HasPrefix(base, ".") {
			return nil
		}

		// ignoring node_modules
		if path != root && info.IsDir() && base == "node_modules" {
			return filepath.SkipDir
		}

		var t string
		switch strings.ToLower(base) {
		case "package-lock.json":
			t = string(depsdev.NPM)
		case "go.mod":
			t = string(depsdev.GO)
		case "pom.xml":
			t = string(depsdev.MAVEN)
		case "cargo.lock":
			t = string(depsdev.CARGO)
		}
		if t != "" {
			if len(types) == 0 || slicez.Contains(types, t) {
				files = append(files, path)
			}
		}

		return nil
	})
	return files

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

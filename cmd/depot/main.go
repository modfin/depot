package main

import (
	"depot/internal/deps"
	"depot/internal/depsdev"
	"fmt"
	"github.com/modfin/henry/slicez"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {

	log.SetLevel(log.ErrorLevel)

	var cache *deps.Cache

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
				Value: "DEPENDENCIES_LICENSE",
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

			return nil
		},
		After: func(context *cli.Context) error {
			return cache.Save()
		},

		Commands: []*cli.Command{
			{
				Name: "print",
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

					l := deps.ToLicense(c.String("root"), allDeps)

					fmt.Println(l.String())

					return nil
				},
			},
			{
				Name: "save",
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

					l := deps.ToLicense(c.String("root"), allDeps)

					rootdir := c.String("root")
					outname := c.String("license-file")

					return os.WriteFile(filepath.Join(rootdir, outname), []byte(l.String()), 0644)
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

					l := deps.ToLicense(c.String("root"), allDeps)

					fmt.Println(l.String())

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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

package main

import (
	"depot/internal/deps"
	"depot/internal/depsdev"
	"fmt"
	"github.com/modfin/henry/slicez"
	"github.com/urfave/cli/v2"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	app := &cli.App{
		Name:  "depot",
		Usage: "a dep license tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Value: ".",
			},
			&cli.BoolFlag{
				Name:    "recurse",
				Aliases: []string{"r"},
			},
		},

		Commands: []*cli.Command{
			{
				Name: "license",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:        "type",
						Usage:       "Type of dep files we are looking for, go, npm, maven, cargo",
						DefaultText: "All",
						Aliases:     []string{"t"},
					},
				},
				Subcommands: []*cli.Command{
					{
						Name: "print",
						Action: func(c *cli.Context) error {
							files := filesFromCli(c)

							var allDeps []deps.Dep

							for _, file := range files {
								d, err := deps.FromFile(file)
								if err != nil {
									fmt.Println("could not resolve", file, "error:", err)
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
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func filesFromCli(c *cli.Context) []string {

	if c.Args().Len() > 0 {
		return c.Args().Slice()
	}

	return findfiles(c.String("root"), c.Bool("recurse"), c.StringSlice("type"))
}

func findfiles(root string, recurse bool, types []string) []string {
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

		//fmt.Println("Looking at", path)

		var t string
		switch strings.ToLower(base) {
		case "package-lock.json":
			t = string(depsdev.NPM)
		case "go.mod":
			t = string(depsdev.GO)
		case "pom.xml":
			t = string(depsdev.MAVEN)
		case "cargo.toml":
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

package main

import (
	"flag"
	"fmt"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/src-d/go-billy.v4"
)

func main() {
	var flags struct {
		tileName   string
		inputPath  string
		outputPath string
	}

	flag.StringVar(&flags.tileName, "tile-name", "", "name of tile product")
	flag.StringVar(&flags.inputPath, "input-path", "", "path to metadata parts directory")
	flag.StringVar(&flags.outputPath, "output-path", "", "path to output directory")
	flag.Parse()

	if flags.tileName == "" {
		log.Fatalln("please provide a tile name using the --tile-name option")
	}

	if flags.inputPath == "" {
		log.Fatalln("please provide a metadata parts directory path using the --input-path option")
	}

	if flags.outputPath == "" {
		log.Fatalln("please provide an output directory path using the --output-path option")
	}

	err := Run(osfs.New(flags.outputPath), osfs.New(flags.inputPath), flags.tileName, []string{"ert", "srt"})
	if err != nil {
		log.Fatalln(err)
	}
}

func Run(out, in billy.Filesystem, currentTileName string, tileNames []string) error {
	return Walk(in, "", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && (info.Name() == "vendor" || info.Name() == "fixtures") {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".yml" {
			return nil
		}

		inFile, err := in.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = inFile.Close()
		}()

		err = out.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			return err
		}

		outFile, err := out.Create(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = outFile.Close()
		}()

		err = processTemplateFile(outFile, inFile, currentTileName, tileNames)
		if err != nil {
			return err
		}

		return nil
	})
}

func processTemplateFile(out io.Writer, in io.Reader, tileName string, tileNames []string) error {
	metadataFile, err := ioutil.ReadAll(in)
	if err != nil {
		return err
	}

	tmpl, err := template.New("tile-preprocess").
		Funcs(helperFuncs(tileName, tileNames)).
		Option("missingkey=error").
		Parse(string(metadataFile))
	if err != nil {
		return err
	}

	err = tmpl.Execute(out, nil)
	if err != nil {
		return err
	}

	return nil
}

func helperFuncs(currentTile string, tileNames []string) template.FuncMap {
	funcs := make(template.FuncMap)

	funcs["tile"] = func() (interface{}, error) {
		for _, n := range tileNames {
			if n == currentTile {
				return currentTile, nil
			}
		}

		return "", fmt.Errorf("unsupported tile name: %s\n", currentTile)
	}

	createPipeSwitchForTile := func(name string) func(value interface{}, tile interface{}) interface{} {
		return func(value interface{}, tile interface{}) interface{} {
			if tile == name {
				return value
			}

			return tile
		}
	}

	for _, n := range tileNames {
		funcs[n] = createPipeSwitchForTile(n)
	}

	return funcs
}

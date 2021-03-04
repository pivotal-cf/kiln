package preprocess

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/src-d/go-billy.v4"
)

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

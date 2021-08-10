package preprocess

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-git/go-billy/v5"
)

func Run(out, in billy.Filesystem, currentTileName string, tileNames []string) error {
	if currentTileName == "" {
		return fmt.Errorf("tile name must not be empty")
	}
	tileNames = filterAndCleanTileNames(tileNames)

	found := false
	for _, n := range tileNames {
		err := isValidTemplateFunctionIdentifier(n)
		if err != nil {
			return err
		}
		if n == currentTileName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%q is not provided in tile names list %v", currentTileName, tileNames)
	}

	outBuf := new(bytes.Buffer)

	return Walk(in, "", func(path string, info os.FileInfo, err error) error {
		defer outBuf.Reset()

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

		err = processTemplateFile(outBuf, inFile, currentTileName, tileNames)
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

		_, err = outBuf.WriteTo(outFile)
		if err != nil {
			return err
		}

		return nil
	})
}

func filterAndCleanTileNames(names []string) []string {
	filtered := names[:0]
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		filtered = append(filtered, n)
	}
	return filtered
}

func isValidTemplateFunctionIdentifier(str string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%q is not a valid identifier", str)
		}
	}()

	_, err = template.New("").Funcs(map[string]interface{}{str: func() string { return "" }}).Parse(fmt.Sprintf("{{%s}}", str))
	return err
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

	funcs["tile"] = func() string { return currentTile }

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

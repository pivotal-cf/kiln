package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"text/template"
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

	err := filepath.Walk(flags.inputPath, filepath.WalkFunc(func(path string, info os.FileInfo, err error) error {
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

		outputFile, err := filepath.Rel(flags.inputPath, path)
		if err != nil {
			return err // Not tested
		}

		err = processTemplateFile(path, filepath.Join(flags.outputPath, outputFile), flags.tileName)
		if err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		log.Fatalln(err)
	}
}

func processTemplateFile(inputFilePath, outputFilePath, tileName string) error {
	metadataFile, err := ioutil.ReadFile(inputFilePath)
	if err != nil {
		return err
	}

	tmpl, err := template.New("tile-preprocess").
		Funcs(template.FuncMap(map[string]interface{}{
			"tile": func() (interface{}, error) {
				if tileName != "ert" && tileName != "srt" {
					return "", fmt.Errorf("unsupported tile name: %s\n", tileName)
				}
				return tileName, nil
			},
			"ert": func(value interface{}, tile interface{}) interface{} {
				if tile == "ert" {
					return value
				}

				return tile
			},
			"srt": func(value interface{}, tile interface{}) interface{} {
				if tile == "srt" {
					return value
				}

				return tile
			},
		})).
		Option("missingkey=error").
		Parse(string(metadataFile))
	if err != nil {
		return err
	}

	dir := filepath.Dir(outputFilePath)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.Fatalln(err) // Not tested
		}
	}()

	err = tmpl.Execute(file, nil)
	if err != nil {
		return err
	}

	return nil
}

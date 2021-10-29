package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/preprocess"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type PreProcess struct {
	Options struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`

		TileName   string `short:"n" long:"tile-name" description:"name of tile to pre-process"`
		InputPath  string `short:"i" long:"input-path" default:"." description:"path to metadata parts directory"`
		OutputPath string `short:"o" long:"output-path" default:"." description:"path to output directory"`

		TileNames string `short:"tn" long:"tile-names" description:"a comma separated list of tile names"`
	}
}

func (p PreProcess) Execute(args []string) error {
	_, err := jhanda.Parse(&p.Options, args)
	if err != nil {
		return err
	}

	kilnFile, err := os.Open(p.Options.Kilnfile)
	if err != nil {
		return err
	}
	defer func() {
		_ = kilnFile.Close()
	}()

	var kilnfile cargo.Kilnfile
	if err := yaml.NewDecoder(kilnFile).Decode(&kilnfile); err != nil {
		return fmt.Errorf("could not parse Kilnfile: %s", err)
	}
	if len(kilnfile.TileNames) == 0 {
		kilnfile.TileNames = strings.Split(p.Options.TileNames, ",")
	}

	err = preprocess.Run(osfs.New(p.Options.OutputPath), osfs.New(p.Options.InputPath), p.Options.TileName, kilnfile.TileNames)
	if err != nil {
		return err
	}

	return nil
}

func (p PreProcess) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Preprocesses yaml files using Go template and adds some helper functions.",
		ShortDescription: "preprocess yaml files",
		Flags:            p.Options,
	}
}

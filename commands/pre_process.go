package commands

import (
	"fmt"
	cargo2 "github.com/pivotal-cf/kiln/pkg/cargo"
	"os"
	"strings"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/preprocess"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/yaml.v2"
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

func (cmd PreProcess) Execute(args []string) error {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return err
	}

	kilnFile, err := os.Open(cmd.Options.Kilnfile)
	if err != nil {
		return err
	}
	defer func() {
		_ = kilnFile.Close()
	}()

	var kilnfile cargo2.Kilnfile
	if err := yaml.NewDecoder(kilnFile).Decode(&kilnfile); err != nil {
		return fmt.Errorf("could not parse Kilnfile: %s", err)
	}
	if len(kilnfile.TileNames) == 0 {
		kilnfile.TileNames = strings.Split(cmd.Options.TileNames, ",")
	}

	err = preprocess.Run(osfs.New(cmd.Options.OutputPath), osfs.New(cmd.Options.InputPath), cmd.Options.TileName, kilnfile.TileNames)
	if err != nil {
		return err
	}

	return nil
}

func (cmd PreProcess) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Preprocesses yaml files using Go template and adds some helper functions.",
		ShortDescription: "preprocess yaml files",
		Flags:            cmd.Options,
	}
}

package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/carvel"
)

type CarvelBake struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelBakeOptions
}

type CarvelBakeOptions struct {
	SourceDirectory string `short:"s" long:"source-directory" description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile      string `short:"o" long:"output-file"      description:"path to where the tile will be output" required:"true"`
	Verbose         bool   `short:"v" long:"verbose"          description:"enable verbose output"`
}

func NewCarvelBake(outLogger, errLogger *log.Logger) CarvelBake {
	return CarvelBake{
		outLogger: outLogger,
		errLogger: errLogger,
	}
}

func (c CarvelBake) Execute(args []string) error {
	_, err := jhanda.Parse(&c.Options, args)
	if err != nil {
		return err
	}

	sourcePath := c.Options.SourceDirectory
	if sourcePath == "" {
		sourcePath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		sourcePath, err = filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve source directory: %w", err)
		}
	}

	targetPath, err := filepath.Abs(c.Options.OutputFile)
	if err != nil {
		return fmt.Errorf("failed to resolve output file path: %w", err)
	}

	baker := carvel.NewBaker()
	if c.Options.Verbose {
		baker.SetWriter(os.Stdout)
	}

	c.outLogger.Printf("Baking Carvel tile from %s into %s/.ezbake", sourcePath, sourcePath)
	err = baker.Bake(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to prepare Carvel tile: %w", err)
	}

	v, err := baker.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get tile version: %w", err)
	}

	err = baker.KilnBake(targetPath)
	if err != nil {
		return fmt.Errorf("failed to bake tile: %w", err)
	}

	c.outLogger.Printf("Baked %s version %s to %s", baker.GetName(), v, targetPath)
	return nil
}

func (c CarvelBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Bakes a Carvel/Kubernetes tile into a .pivotal file. This command transforms a Kubernetes tile (using imgpkg bundles and Carvel packages) into a BOSH-compatible format, then bakes it into a .pivotal file that can be consumed by Operations Manager.",
		ShortDescription: "bakes a Carvel/Kubernetes tile",
		Flags:            c.Options,
	}
}

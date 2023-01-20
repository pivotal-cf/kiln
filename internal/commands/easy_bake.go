package commands

import (
	"log"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

type EasyBake struct {
	Options struct {
		flags.Standard
	}
	logger  *log.Logger
	fs 	billy.Filesystem
	releasesService baking.ReleasesService
	fetcher Fetch 
}

func NewEasyBake(logger *log.Logger, fs billy.Filesystem, releaseService baking.ReleasesService) EasyBake {
	return EasyBake{
		logger: logger,
		fs: fs,
		releasesService: releaseService,
	}
}

func (e EasyBake) Execute(args []string) error {
	e.logger.Println("Hello, EasyBake")

	args = append(args, "--metadata", "base.yml", "--variables-file", "variables.yml")
	err := NewBake(e.fs, e.releasesService, e.logger, e.logger).Execute(args)

	return err
}

func (e EasyBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command is used to bake a release",
		ShortDescription: "bakes a release",
		Flags:            e.Options,
	}
}

package commands

import (
	"log"

	"github.com/pivotal-cf/jhanda"
)

type Version struct {
	logger  *log.Logger
	version string
}

func NewVersion(logger *log.Logger, version string) Version {
	return Version{
		logger:  logger,
		version: version,
	}
}

func (v Version) Execute([]string) error {
	v.logger.Printf("kiln version %s\n", v.version)

	return nil
}

func (v Version) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command prints the kiln release version number.",
		ShortDescription: "prints the kiln release version",
	}
}

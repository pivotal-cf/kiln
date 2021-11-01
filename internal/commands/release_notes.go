package commands

import (
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

type ReleaseNotes struct{
	Options struct {
		flags.Standard
	}
}

func (r ReleaseNotes) Execute(args []string) error {
	return nil
}

func (r ReleaseNotes) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description: "generates release notes from bosh-release release notes on GitHub between two tile repo git references",
		ShortDescription: "generates release notes from bosh-release release notes",
		Flags: r.Options,
	}
}




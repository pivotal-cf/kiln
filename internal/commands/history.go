package commands

import (
	"github.com/pivotal-cf/jhanda"

	"github.com/go-git/go-billy/v5"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

type History struct {
	Options struct {
		flags.Standard
	}

	FS billy.Filesystem
}

func (cmd History) Execute(args []string) error {
	panic("implement me")
}

func (cmd History) Usage() jhanda.Usage {
	panic("implement me")
}

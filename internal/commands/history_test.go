package commands_test

import (
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands"
)

var _ jhanda.Command = (*commands.History)(nil)

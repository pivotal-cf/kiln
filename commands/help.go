package commands

import (
	"github.com/jessevdk/go-flags"
)

type HelpCmd struct {
	root *flags.Parser
	panicCommand
}

func (h HelpCmd) Runner(deps Dependencies) (CommandRunner, error) {
	h.root = deps.RootCmd
	return h, nil
}

func (h HelpCmd) Run(args []string) error {
	_, err := h.root.ParseArgs(append(args, "--help"))
	return err
}


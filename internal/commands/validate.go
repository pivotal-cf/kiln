package commands

import (
	"fmt"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Validate struct {
	Options struct {
		flags.Standard
		ReleaseSourceTypeAllowList []string `long:"allow-release-source-type"`
	}

	FS billy.Filesystem
}

var _ jhanda.Command = (*Validate)(nil)

func NewValidate(fs billy.Filesystem) Validate {
	return Validate{
		FS: fs,
	}
}

func (v Validate) Execute(args []string) error {
	_, err := flags.LoadWithDefaultFilePaths(&v.Options, args, v.FS.Stat)
	if err != nil {
		return err
	}

	kf, lock, err := v.Options.Standard.LoadKilnfiles(v.FS, nil)
	if err != nil {
		return fmt.Errorf("failed to load kilnfiles: %w", err)
	}

	errs := cargo.Validate(kf, lock, cargo.ValidateResourceTypeAllowList(v.Options.ReleaseSourceTypeAllowList...))
	if len(errs) > 0 {
		return errorList(errs)
	}

	return nil
}

type errorList []error

func (list errorList) Error() string {
	messages := make([]string, 0, len(list))
	for _, err := range list {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

func (v Validate) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Validate checks for common Kilnfile and Kilnfile.lock mistakes",
		ShortDescription: "validate Kilnfile and Kilnfile.lock",
		Flags:            v.Options,
	}
}

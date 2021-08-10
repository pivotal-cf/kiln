package commands

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Validate struct {
	Options struct {
		flags.Standard
	}

	FS billy.Filesystem
}

var _ jhanda.Command = (*Validate)(nil)

func (v Validate) Execute(args []string) error {
	err := flags.LoadFlagsWithDefaults(&v.Options, args, v.FS.Stat)
	if err != nil {
		return err
	}

	kilnfile, _, err := v.Options.Standard.LoadKilnfiles(v.FS, nil)
	if err != nil {
		return fmt.Errorf("failed to load kilnfiles: %w", err)
	}

	var releaseErrors errorList

	for index, release := range kilnfile.Releases {
		if err := validateRelease(release, index); err != nil {
			releaseErrors = append(releaseErrors, err)
		}
	}

	if len(releaseErrors) > 0 {
		return releaseErrors
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

func validateRelease(release cargo.ReleaseKiln, index int) error {
	if release.Name == "" {
		return fmt.Errorf("release at index %d missing name", index)
	}

	if release.Version == "" {
		return nil
	}

	if _, err := semver.NewConstraint(release.Version); err != nil {
		return fmt.Errorf("release %s (index %d) has invalid version constraint: %w", release.Name, index, err)
	}

	return nil
}

func (v Validate) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Validate checks for common Kilnfile and Kilnfile.lock mistakes",
		ShortDescription: "validate Kilnfile and Kilnfile.lock",
		Flags:            v.Options,
	}
}

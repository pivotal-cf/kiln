package commands

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Validate struct {
	Options struct {
		Kilnfile       string   `short:"kf" long:"kilnfile" default:"Kilnfile" description:"path to Kilnfile"`
		VariablesFiles []string `short:"vf" long:"variables-file" description:"path to variables file"`
		Variables      []string `short:"vr" long:"variable" description:"variable in key=value format"`
	}

	FS billy.Filesystem
}

var _ jhanda.Command = (*Validate)(nil)

func (v Validate) Execute(args []string) error {
	if v.FS == nil {
		v.FS = osfs.New("")
	}

	_, err := jhanda.Parse(&v.Options, args)
	if err != nil {
		return fmt.Errorf("failed to parse comand arguments: %w", err)
	}

	kilnfile, _, err := cargo.KilnfileLoader{}.LoadKilnfiles(v.FS,
		v.Options.Kilnfile,
		v.Options.VariablesFiles,
		v.Options.Variables,
	)
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

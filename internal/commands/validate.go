package commands

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Validate struct {
	Options struct {
		options.Standard
	}

	FS billy.Filesystem
}

func NewValidate(fs billy.Filesystem) Validate {
	return Validate{
		FS: fs,
	}
}

func (v Validate) Execute(args []string) error {
	return Kiln{
		Wrapped: v,
		KilnfileStore: KilnfileStore{
			FS: v.FS,
		},
		StatFn: v.FS.Stat,
	}.Execute(args)
}

func (v Validate) KilnExecute(args []string, parseOps OptionsParseFunc) error {
	kilnfile, kilnfileLock, _, err := parseOps(args, &v.Options)
	if err != nil {
		return err
	}

	var releaseErrors errorList

	for index, release := range kilnfile.Releases {
		releaseLock, err := kilnfileLock.FindReleaseWithName(release.Name)
		if err != nil {
			releaseErrors = append(releaseErrors,
				fmt.Errorf("release %q not found in kilnfileLock", release.Name))
			continue
		}

		if err := validateRelease(release, releaseLock, index); err != nil {
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

func validateRelease(release cargo.ComponentSpec, lock cargo.ComponentLock, index int) error {
	if release.Name == "" {
		return fmt.Errorf("release at index %d missing name", index)
	}

	if release.Version != "" {
		c, err := semver.NewConstraint(release.Version)
		if err != nil {
			return fmt.Errorf("release %s (index %d in Kilnfile) has invalid version constraint: %w",
				release.Name, index, err)
		}

		v, err := semver.NewVersion(lock.Version)
		if err != nil {
			return fmt.Errorf("release %s (index %d in Kilnfile.lock) has invalid lock version %q: %w",
				release.Name, index, lock.Version, err)
		}

		matches, errs := c.Validate(v)
		if !matches {
			return fmt.Errorf("release %s version in lock %q does not match constraint %q: %v",
				release.Name, lock.Version, release.Version, errs)
		}
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

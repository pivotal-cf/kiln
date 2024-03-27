package cargo

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"
)

func Validate(spec Kilnfile, lock KilnfileLock) []error {
	var result []error

	for index, componentSpec := range spec.Releases {
		if componentSpec.Name == "" {
			result = append(result, fmt.Errorf("release at index %d missing name in spec", index))
			continue
		}

		componentLock, err := lock.FindBOSHReleaseWithName(componentSpec.Name)
		if err != nil {
			result = append(result,
				fmt.Errorf("release %q not found in lock", componentSpec.Name))
			continue
		}

		if err := checkComponentVersionsAndConstraint(componentSpec, componentLock, index); err != nil {
			result = append(result, err)
		}
	}

	for index, componentLock := range lock.Releases {
		if componentLock.Name == "" {
			result = append(result, fmt.Errorf("release at index %d missing name in lock", index))
			continue
		}

		_, err := spec.BOSHReleaseTarballSpecification(componentLock.Name)
		if err != nil {
			result = append(result,
				fmt.Errorf("release %q not found in spec", componentLock.Name))
			continue
		}
	}

	result = append(result, ensureRemoteSourceExistsForEachReleaseLock(spec, lock)...)

	if len(result) > 0 {
		return result
	}

	return nil
}

func ensureRemoteSourceExistsForEachReleaseLock(spec Kilnfile, lock KilnfileLock) []error {
	var result []error
	for _, release := range lock.Releases {
		if releaseSourceIndex := slices.IndexFunc(spec.ReleaseSources, func(config ReleaseSourceConfig) bool {
			return BOSHReleaseTarballSourceID(config) == release.RemoteSource
		}); releaseSourceIndex < 0 {
			result = append(result,
				fmt.Errorf("release source %q for release lock %q not found in Kilnfile", release.RemoteSource, release.Name))
		}
	}
	return result
}

func checkComponentVersionsAndConstraint(spec BOSHReleaseTarballSpecification, lock BOSHReleaseTarballLock, index int) error {
	v, err := semver.NewVersion(lock.Version)
	if err != nil {
		return fmt.Errorf("spec %s (index %d in Kilnfile.lock) has invalid lock version %q: %w",
			spec.Name, index, lock.Version, err)
	}

	if spec.Version != "" {
		c, err := semver.NewConstraint(spec.Version)
		if err != nil {
			return fmt.Errorf("spec %s (index %d in Kilnfile) has invalid version constraint: %w",
				spec.Name, index, err)
		}

		matches, errs := c.Validate(v)
		if !matches {
			return fmt.Errorf("spec %s version in lock %q does not match constraint %q: %v",
				spec.Name, lock.Version, spec.Version, errs)
		}
	}

	return nil
}

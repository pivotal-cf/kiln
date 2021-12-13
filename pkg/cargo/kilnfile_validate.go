package cargo

import (
	"fmt"

	"github.com/Masterminds/semver"
)

func Validate(spec Kilnfile, lock KilnfileLock) []error {
	var result []error

	for index, componentSpec := range spec.Releases {
		if componentSpec.Name == "" {
			result = append(result, fmt.Errorf("spec at index %d missing name", index))
			continue
		}

		componentLock, err := lock.FindReleaseWithName(componentSpec.Name)
		if err != nil {
			result = append(result,
				fmt.Errorf("component spec for release %q not found in lock", componentSpec.Name))
			continue
		}

		if err := checkComponentVersionsAndConstraint(componentSpec, componentLock, index); err != nil {
			result = append(result, err)
		}
	}

	if len(result) > 0 {
		return result
	}

	return nil
}

func checkComponentVersionsAndConstraint(spec ComponentSpec, lock ComponentLock, index int) error {
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

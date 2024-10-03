package cargo

import (
	"fmt"
	"slices"
	"text/template/parse"

	"github.com/Masterminds/semver/v3"
)

type ValidationOptions struct {
	resourceTypeAllowList []string
}

func NewValidateOptions() ValidationOptions {
	return ValidationOptions{}
}

// ValidateResourceTypeAllowList calls ValidationOptions.SetValidateResourceTypeAllowList on the result of NewValidateOptions
func ValidateResourceTypeAllowList(allowList ...string) ValidationOptions {
	return NewValidateOptions().SetValidateResourceTypeAllowList(allowList)
}

func (o ValidationOptions) SetValidateResourceTypeAllowList(allowList []string) ValidationOptions {
	o.resourceTypeAllowList = allowList
	return o
}

func mergeOptions(options []ValidationOptions) ValidationOptions {
	var opt ValidationOptions
	for _, o := range options {
		if o.resourceTypeAllowList != nil {
			opt.resourceTypeAllowList = o.resourceTypeAllowList
		}
	}
	return opt
}

func Validate(spec Kilnfile, lock KilnfileLock, options ...ValidationOptions) []error {
	opt := mergeOptions(options)
	var result []error

	if len(opt.resourceTypeAllowList) > 0 {
		for _, s := range spec.ReleaseSources {
			if !slices.Contains(opt.resourceTypeAllowList, s.Type) {
				result = append(result, fmt.Errorf("release source type not allowed: %s", s.Type))
			}
		}
	}

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
	result = append(result, ensureReleaseSourceConfiguration(spec.ReleaseSources)...)

	if len(result) > 0 {
		return result
	}

	return nil
}

func ensureReleaseSourceConfiguration(sources []ReleaseSourceConfig) []error {
	var errs []error
	for _, source := range sources {
		switch source.Type {
		case BOSHReleaseTarballSourceTypeArtifactory:
			if source.ArtifactoryHost == "" {
				errs = append(errs, fmt.Errorf("missing required field artifactory_host"))
			}
			if source.Username == "" {
				errs = append(errs, fmt.Errorf("missing required field username"))
			}
			if source.Password == "" {
				errs = append(errs, fmt.Errorf("missing required field password"))
			}
			if source.Repo == "" {
				errs = append(errs, fmt.Errorf("missing required field repo"))
			}
			if source.PathTemplate == "" {
				errs = append(errs, fmt.Errorf("missing required field path_template"))
			} else {
				p := parse.New("path_template")
				p.Mode |= parse.SkipFuncCheck
				if _, err := p.Parse(source.PathTemplate, "", "", make(map[string]*parse.Tree)); err != nil {
					errs = append(errs, fmt.Errorf("failed to parse path_template: %w", err))
				}
			}
			if source.Bucket != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field bucket"))
			}
			if source.Region != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field region"))
			}
			if source.AccessKeyId != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field access_key_id"))
			}
			if source.SecretAccessKey != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field secret_access_key"))
			}
			if source.RoleARN != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field role_arn"))
			}
			if source.Endpoint != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field endpoint"))
			}
			if source.Org != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field org"))
			}
			if source.GithubToken != "" {
				errs = append(errs, fmt.Errorf("artifactory has unexpected field github_token"))
			}
		case BOSHReleaseTarballSourceTypeBOSHIO:
		case BOSHReleaseTarballSourceTypeS3:
		case BOSHReleaseTarballSourceTypeGithub:
		}
	}
	return errs
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

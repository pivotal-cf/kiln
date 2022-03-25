package cargo

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
)

const (
	// ReleaseSourceTypeBOSHIO is the value of the Type field on cargo.ReleaseSourceConfig
	// for fetching https://bosh.io releases.
	ReleaseSourceTypeBOSHIO = "bosh.io"

	// ReleaseSourceTypeS3 is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on
	ReleaseSourceTypeS3 = "s3"

	// ReleaseSourceTypeGithub is the value for the Type field on cargo.ReleaseSourceConfig
	// for releases stored on Github.
	ReleaseSourceTypeGithub = "github"
)

type ReleaseSource interface {
	ID() string
	IsPublishable() bool
	Type() string
}

type ReleaseSourceList []ReleaseSource

func (list *ReleaseSourceList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var sources []struct {
		Type   string                 `yaml:"type"`
		Values map[string]interface{} `yaml:",inline"`
	}

	err := unmarshal(&sources)
	if err != nil {
		return err
	}

	for index, element := range sources {
		buf, err := yaml.Marshal(element.Values)
		if err != nil {
			return fmt.Errorf("failed to marshal release source values for release source at index %d: %w", index, err)
		}

		var (
			rs       ReleaseSource
			parseErr error
		)
		switch element.Type {
		case ReleaseSourceTypeBOSHIO:
			s := new(BOSHIOReleaseSource)
			parseErr = yaml.Unmarshal(buf, s)
			rs = *s
		case ReleaseSourceTypeS3, "":
			s := new(S3ReleaseSource)
			parseErr = yaml.Unmarshal(buf, s)
			rs = *s
		case ReleaseSourceTypeGithub:
			s := new(GitHubReleaseSource)
			parseErr = yaml.Unmarshal(buf, s)
			rs = *s
		default:
			return fmt.Errorf("unknown release source type at index %d got type name %s", index, element.Type)
		}
		if parseErr != nil {
			return fmt.Errorf("failed to parse release source type at index %d: %w", index, parseErr)
		}

		*list = append(*list, rs)
	}

	return nil
}

func (list ReleaseSourceList) FindWithID(id string) ReleaseSource {
	for _, rs := range list {
		if rs.ID() == id {
			return rs
		}
	}
	return nil
}

func (list ReleaseSourceList) Validate() error {
	if err := errorIfDuplicateIDs(list); err != nil {
		return err
	}
	return nil
}

func errorIfDuplicateIDs(releaseSources []ReleaseSource) error {
	indexOfID := make(map[string]int)
	for index, rs := range releaseSources {
		id := rs.ID()
		previousIndex, seen := indexOfID[id]
		if seen {
			return fmt.Errorf(`release_sources must have unique IDs; items at index %d and %d both have ID %q`, previousIndex, index, id)
		}
		indexOfID[id] = index
	}
	return nil
}

func (list ReleaseSourceList) Filter(allowOnlyPublishable bool) ReleaseSourceList {
	var sources ReleaseSourceList
	for _, source := range list {
		if allowOnlyPublishable && !source.IsPublishable() {
			continue
		}
		sources = append(sources, source)
	}
	return sources
}

type TemplateVariables = map[string]interface{}

func (list ReleaseSourceList) ConfigureSecrets(tv TemplateVariables) (ReleaseSourceList, ReleaseSourceList, []error) {
	var (
		successful, failed ReleaseSourceList
		failedErrs         []error
	)
	for _, source := range list {
		sourceUsingSecrets, ok := source.(interface {
			ConfigureSecrets(secrets TemplateVariables) (ReleaseSource, error)
		})
		if !ok {
			successful = append(successful, source)
			continue
		}
		src, err := sourceUsingSecrets.ConfigureSecrets(tv)
		if err != nil {
			failed = append(failed, source)
			failedErrs = append(failedErrs, err)
			continue
		}
		successful = append(successful, src)
	}
	successful = successful[:len(successful):len(successful)]
	failed = failed[:len(failed):len(failed)]
	failedErrs = failedErrs[:len(failedErrs):len(failedErrs)]
	return successful, failed, failedErrs
}

func configureSecret(value, name, envVarName string, tv TemplateVariables) (string, error) {
	errPrefix := fmt.Sprintf("configuring secret %s failed: ", name)

	if value == "" {
		value = os.Getenv(envVarName)
		if value == "" {
			return "", fmt.Errorf(errPrefix+"%s is not set", envVarName)
		}
		value = strings.TrimSpace(value)
		return value, nil
	}

	if !strings.Contains(value, builder.InterpolatorLeadingDelimiter) {
		return value, nil
	}

	interpolated, intErr := interpolateSecretes([]byte(value), name, tv)
	if intErr != nil {
		return "", fmt.Errorf(errPrefix+"interpolation failed: %w", intErr)
	}
	value = string(interpolated)
	value = strings.TrimSpace(value)
	return value, nil
}

func interpolateSecretes(in []byte, name string, templateVariables map[string]interface{}) ([]byte, error) {
	interpolator := builder.NewInterpolator()
	interpolated, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, name, in)
	if err != nil {
		return nil, err
	}
	return interpolated, nil
}

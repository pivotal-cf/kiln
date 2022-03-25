package cargo

import (
	"fmt"

	"gopkg.in/yaml.v2"
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
			return fmt.Errorf("failed to marshal release source values: %w", err)
		}

		var rs ReleaseSource
		switch element.Type {
		case ReleaseSourceTypeBOSHIO:
			s := new(BOSHIOReleaseSource)
			err := yaml.Unmarshal(buf, s)
			if err != nil {
				return fmt.Errorf("failed to unmarshal release source at index %d: %w", index, err)
			}
			rs = *s
		case ReleaseSourceTypeS3, "":
			s := new(S3ReleaseSource)
			err := yaml.Unmarshal(buf, s)
			if err != nil {
				return fmt.Errorf("failed to unmarshal release source at index %d: %w", index, err)
			}
			rs = *s
		case ReleaseSourceTypeGithub:
			s := new(GitHubReleaseSource)
			err := yaml.Unmarshal(buf, s)
			if err != nil {
				return fmt.Errorf("failed to unmarshal release source at index %d: %w", index, err)
			}
			rs = *s
		default:
			return fmt.Errorf("unknown release source type: %q", element.Type)
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

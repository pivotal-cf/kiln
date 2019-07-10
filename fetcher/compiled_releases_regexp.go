package fetcher

import (
	"fmt"
	"regexp"
)

const (
	ReleaseName     = "release_name"
	ReleaseVersion  = "release_version"
	StemcellOS      = "stemcell_os"
	StemcellVersion = "stemcell_version"
)

type CompiledReleasesRegexp struct {
	r *regexp.Regexp
}

func NewCompiledReleasesRegexp(regex string) (*CompiledReleasesRegexp, error) {
	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}

	var count int
	for _, name := range r.SubexpNames() {
		if name == ReleaseName || name == ReleaseVersion || name == StemcellOS || name == StemcellVersion {
			count++
		}
	}
	if count != 4 {
		return nil, fmt.Errorf("Missing some capture group. Required capture groups: %s, %s, %s, %s", ReleaseName, ReleaseVersion, StemcellOS, StemcellVersion)
	}

	return &CompiledReleasesRegexp{r: r}, nil
}

func (crr *CompiledReleasesRegexp) Convert(s3Key string) (CompiledRelease, error) {
	if !crr.r.MatchString(s3Key) {
		return CompiledRelease{}, fmt.Errorf("s3 key does not match regex")
	}

	matches := crr.r.FindStringSubmatch(s3Key)
	subgroup := make(map[string]string)
	for i, name := range crr.r.SubexpNames() {
		if i != 0 && name != "" {
			subgroup[name] = matches[i]
		}
	}

	return CompiledRelease{
		Name:            subgroup[ReleaseName],
		Version:         subgroup[ReleaseVersion],
		StemcellOS:      subgroup[StemcellOS],
		StemcellVersion: subgroup[StemcellVersion],
	}, nil
}

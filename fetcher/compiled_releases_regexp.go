package fetcher

import (
	"fmt"
	"regexp"
)

type ReleasesRegexp struct {
	r *regexp.Regexp
}

func NewReleasesRegexp(regex string) (*ReleasesRegexp, error) {
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

	return &ReleasesRegexp{r: r}, nil
}

func (crr *ReleasesRegexp) Convert(s3Key string) (CompiledRelease, error) {
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
		ID: ReleaseID{
			Name:    subgroup[ReleaseName],
			Version: subgroup[ReleaseVersion],
		},
		StemcellOS:      subgroup[StemcellOS],
		StemcellVersion: subgroup[StemcellVersion],
		localPath:       "",
	}, nil
}

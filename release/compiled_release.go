package release

import (
	"fmt"
)

type compiledRelease struct {
	builtRelease
	StemcellOS      string
	StemcellVersion string
}

func NewCompiledRelease(id ReleaseID, stemcellOS, stemcellVersion string) releaseWithLocation {
	return releaseWithLocation{
		unhomedRelease: compiledRelease{
			builtRelease:     builtRelease(id),
			StemcellOS:      stemcellOS,
			StemcellVersion: stemcellVersion,
		},
	}
}

func (cr compiledRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s-%s-%s.tgz", cr.Name, cr.Version, cr.StemcellOS, cr.StemcellVersion)
}
func (cr compiledRelease) Satisfies(rr ReleaseRequirement) bool {
	return cr.builtRelease.Satisfies(rr) &&
		cr.StemcellOS == rr.StemcellOS &&
		cr.StemcellVersion == rr.StemcellVersion
}

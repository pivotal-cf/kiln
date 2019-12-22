package release

import (
	"fmt"
)

type builtRelease ReleaseID

func NewBuiltRelease(id ReleaseID) releaseWithLocation {
	return releaseWithLocation{unhomedRelease: builtRelease(id)}
}

func (br builtRelease) StandardizedFilename() string {
	return fmt.Sprintf("%s-%s.tgz", br.Name, br.Version)
}

func (br builtRelease) Satisfies(rr ReleaseRequirement) bool {
	return br.Name == rr.Name &&
		br.Version == rr.Version
}

func (br builtRelease) ReleaseID() ReleaseID {
	return ReleaseID(br)
}

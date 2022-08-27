package cargo

import "github.com/Masterminds/semver"

// version should only be updated when the Kiln major version changes
// it can be set via ldflags like this:
//   go run -ldflags "-X github.com/pivotal-cf/kiln/pkg/cargo.version=1899.11.29-fc.barca" github.com/pivotal-cf/kiln -- version
var version = "0.0.0-dev"

func Version() *semver.Version {
	return semver.MustParse(version)
}

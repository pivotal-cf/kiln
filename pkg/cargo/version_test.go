package cargo

import (
	"github.com/Masterminds/semver"
	"testing"
)

func Test_version(t *testing.T) {
	v := semver.MustParse(version)

	if v.Minor() != 0 {
		t.Error("minor should not be set")
	}
	if v.Patch() != 0 {
		t.Error("patch should not be set")
	}
}

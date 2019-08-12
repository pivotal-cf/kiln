package commands

import (
	"fmt"
	"testing"

	"github.com/Masterminds/semver"
)

func TestInternal_releaseType(t *testing.T) {
	test := func(window, version, expected string) {
		t.Run(fmt.Sprintf("when the window is %q and the version is %q", window, version), func(t *testing.T) {
			if str := releaseType(window, semver.MustParse(version)); string(str) != expected {
				t.Errorf("it should return %q", expected)
				t.Logf("got: %q", str)
			}
		})
	}

	test("rc", "2.0.0", "Release Candidate")
	test("beta", "2.0.0", "Beta Release")
	test("ga", "2.0.0", "Major Release")
	test("ga", "2.1.0", "Minor Release")
	test("ga", "2.1.1", "Maintenance Release")
	test("ga", "2.1.1-foo.1", "Developer Release")
	test("alpha", "2.0.0", "Alpha Release")
	test("internal", "2.0.0", "Developer Release")
	test("foo", "2.0.0", "Developer Release")
}

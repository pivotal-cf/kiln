package commands

import (
	"io"
	"log"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

const (
	warnReleaseSourceShouldBeSet = "\n\tWARNING release_source SHOULD be set for each release in the Kilnfile.\n\tIn some upcoming kiln release, the release_source field will be required."
)

func getReleaseSource(spec cargo.ComponentSpec, list component.MultiReleaseSource, logger *log.Logger) (interface {
	component.ReleaseVersionFinder
	component.MatchedReleaseGetter
	component.ReleaseDownloader
}, error) {
	if spec.ReleaseSource == "" {
		logger.Printf(warnReleaseSourceShouldBeSet)
		return list, nil
	}
	return list.FindByID(spec.ReleaseSource)
}

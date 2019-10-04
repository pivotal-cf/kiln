package fetcher

import (
	"fmt"
	"log"

	"github.com/pivotal-cf/kiln/internal/cargo"
)

//go:generate counterfeiter -o ./fakes/release_source.go --fake-name ReleaseSource . ReleaseSource
type ReleaseSource interface {
	GetMatchedReleases(assetsLock ReleaseSet, stemcell cargo.Stemcell) (ReleaseSet, error)
	DownloadReleases(releasesDir string, matchedS3Objects ReleaseSet, downloadThreads int) error
}

type releaseSourceFunction func(assets cargo.Assets) []ReleaseSource

func (rsf releaseSourceFunction) ReleaseSources(assets cargo.Assets) []ReleaseSource {
	return rsf(assets)
}

func NewReleaseSourcesFactory(outLogger *log.Logger) releaseSourceFunction {
	return func(assets cargo.Assets) []ReleaseSource {
		var releaseSources []ReleaseSource

		for _, releaseConfig := range assets.ReleaseSources {
			releaseSources = append(releaseSources, releaseSourceFor(releaseConfig, outLogger))
		}

		return releaseSources
	}
}

func releaseSourceFor(releaseConfig cargo.ReleaseSourceConfig, outLogger *log.Logger) ReleaseSource {
	if releaseConfig.Type == "bosh.io" {
		return NewBOSHIOReleaseSource(outLogger, "")
	}

	if releaseConfig.Type != "s3" {
		panic(fmt.Sprintf("unknown release config: %v", releaseConfig))
	}

	s3ReleaseSource := S3ReleaseSource{Logger: outLogger}
	s3ReleaseSource.Configure(releaseConfig)
	if releaseConfig.Compiled {
		return S3CompiledReleaseSource(s3ReleaseSource)
	} else {
		return S3BuiltReleaseSource(s3ReleaseSource)
	}
}

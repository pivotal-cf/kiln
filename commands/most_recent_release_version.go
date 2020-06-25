package commands

import (
	"github.com/pivotal-cf/kiln/fetcher"
	"log"
)

type MostRecentReleaseVersion struct {
	mrsProvider fetcher.MultiReleaseSource
	outLogger *log.Logger
}

func NewMostRecentReleaseVersion(mrsProvider fetcher.MultiReleaseSource, outLogger *log.Logger) MostRecentReleaseVersion {
	return MostRecentReleaseVersion{
		mrsProvider,
		outLogger,
	}
}

func (cmd *MostRecentReleaseVersion) Execute(args []string) error {
	return nil
}
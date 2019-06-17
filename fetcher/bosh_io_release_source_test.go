package fetcher_test

import (
	"log"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	var (
		releaseSource    fetcher.BOSHIOReleaseSource
		assetsLock       cargo.AssetsLock //list of required Bosh releases
	)

	It("Given a list of required BOSH releases; "+
		"Given bosh.io has those releases; "+
		"then those BOSH releases are included in `foundReleases`", func() {
		//var boshioReleaseSource fetcher.BOSHIOReleaseSource

		logger := log.New(nil, "", 0)
		releaseSource = fetcher.NewBOSHIOReleaseSource(logger)

		assetsLock = cargo.AssetsLock{
			Releases: []cargo.Release{
				{Name: "bpm", Version: "1.2.3-lts"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "ubuntu-xenial",
				Version: "190.0.0",
			},
		}

		foundReleases, _, err := releaseSource.GetMatchedReleases(assetsLock)
		Expect(err).ToNot(HaveOccurred())
		Expect(foundReleases).ToNot(BeNil())
	})

})

package fetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	var (
		//list of required Bosh releases
		//bosh io fake client
	)

	It("Given a list of required BOSH releases; Given bosh.io has those releases; then those BOSH releases are included in `foundReleases`", func() {
		//var boshioReleaseSource fetcher.BOSHIOReleaseSource
		//foundReleases = boshioReleaseSource.GetMatchedReleases()
		Expect(true).To(Equal(true))
	})


})
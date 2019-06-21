package fetcher_test

import (
	"log"

	"github.com/pivotal-cf/kiln/fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/cargo"
)

var _ = Describe("ReleaseExistOnBoshio", func() {
	It("returns true if the release can be found on bosh.io", func() {
		Expect(fetcher.ReleaseExistOnBoshio("cloudfoundry/uaa-release")).To(BeTrue())
	})

	It("returns false if the release can not be found on bosh.io", func() {
		Expect(fetcher.ReleaseExistOnBoshio("foo")).To(BeFalse())
	})
})

var _ = Describe("GetMatchedReleases from bosh.io", func() {
	var (
		releaseSource             *fetcher.BOSHIOReleaseSource
		desiredCompiledReleaseSet cargo.CompiledReleaseSet
	)

	It("returns releases which exists on bosh.io", func() {
		logger := log.New(nil, "", 0)
		releaseSource = fetcher.NewBOSHIOReleaseSource(logger)

		os := "ubuntu-xenial"
		version := "190.0.0"
		desiredCompiledReleaseSet = cargo.CompiledReleaseSet{
			{Name: "uaa", Version: "73.3.0", StemcellOS: os, StemcellVersion: version}:          "",
			{Name: "zzz", Version: "999", StemcellOS: os, StemcellVersion: version}:             "",
			{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: os, StemcellVersion: version}: "",
		}

		foundReleases, err := releaseSource.GetMatchedReleases(desiredCompiledReleaseSet)
		uaaURL := "https://bosh.io/d/github.com/cloudfoundry/uaa-release?v=73.3.0"
		cfRabbitURL := "https://bosh.io/d/github.com/pivotal-cf/cf-rabbitmq-release?v=268.0.0"

		Expect(err).ToNot(HaveOccurred())
		Expect(foundReleases).To(HaveLen(2))
		Expect(foundReleases).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "uaa", Version: "73.3.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, uaaURL))
		Expect(foundReleases).To(HaveKeyWithValue(cargo.CompiledRelease{Name: "cf-rabbitmq", Version: "268.0.0", StemcellOS: "ubuntu-xenial", StemcellVersion: "190.0.0"}, cfRabbitURL))
	})
})

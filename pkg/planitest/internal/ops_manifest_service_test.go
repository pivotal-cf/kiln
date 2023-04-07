package internal_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/planitest/internal"
	"github.com/pivotal-cf/kiln/pkg/planitest/internal/fakes"
)

var _ = Describe("OpsManifest Service", func() {
	var (
		opsManifestService *internal.OpsManifestService
		opsManifestRunner  *fakes.OpsManifestRunner
	)

	BeforeEach(func() {
		var err error

		err = os.Setenv("RENDERER", "ops-manifest")
		Expect(err).NotTo(HaveOccurred())

		opsManifestRunner = &fakes.OpsManifestRunner{}
		opsManifestService, err = internal.NewOpsManifestServiceWithRunner(opsManifestRunner, internal.RealIO)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("RenderManifest", func() {
		It("calls ops-manifest to retrieve the manifest", func() {
			opsManifestRunner.GetManifestReturns(map[string]interface{}{
				"some-key": "some-value",
			}, nil)

			manifest, err := opsManifestService.RenderManifest(strings.NewReader(`---
network-properties:
	network:
		name: some-network

product-properties:
	.some-minimal-config:
		value: some-value
`), strings.NewReader(""))
			Expect(err).NotTo(HaveOccurred())

			Expect(opsManifestRunner.GetManifestCallCount()).To(Equal(1))

			Expect(manifest).To(MatchYAML(`some-key: some-value`))
		})
	})
})

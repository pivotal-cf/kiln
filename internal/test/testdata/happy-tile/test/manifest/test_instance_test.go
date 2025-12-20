package manifest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cf CLI", func() {
	Context("cf CLI v6", func() {
		It("colocates the cf-cli-6-linux job on the instance group used to run errands", func() {
			manifest, err := product.RenderManifest(nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = manifest.FindInstanceGroupJob("my-instance-group", "my-errand")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

package proofing_test

import (
	"github.com/pivotal-cf/kiln/internal/proofing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Constraint", func() {
	var constraint proofing.Constraint

	BeforeEach(func() {
		productTemplate, err := proofing.Parse("fixtures/metadata.yml")
		Expect(err).NotTo(HaveOccurred())

		constraint = productTemplate.JobTypes[0].InstanceDefinition.Constraints
	})

	It("parses their structure", func() {
		Expect(constraint.Min).To(Equal(1))
		Expect(constraint.Max).To(Equal(5))
		Expect(constraint.MustMatchRegex).To(Equal("some-must-match-regex"))
		Expect(constraint.ErrorMessage).To(Equal("some-error-message"))
	})
})

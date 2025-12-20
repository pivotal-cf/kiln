package proofing_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

var _ = Describe("IntegerConstraints", func() {
	DescribeTable("CheckValue",
		func(constraint proofing.IntegerConstraints, value int, matcher OmegaMatcher) {
			Expect(constraint.CheckValue(value)).To(matcher)
		},
		Entry("below min", proofing.IntegerConstraints{Min: ptr(3)}, 1, MatchError(ContainSubstring("greater than"))),
		Entry("above max", proofing.IntegerConstraints{Max: ptr(3)}, 5, MatchError(ContainSubstring("less than"))),
		Entry("at min", proofing.IntegerConstraints{Min: ptr(3)}, 3, Not(HaveOccurred())),
		Entry("at max", proofing.IntegerConstraints{Max: ptr(3)}, 3, Not(HaveOccurred())),
		Entry("between min and max", proofing.IntegerConstraints{Min: ptr(4), Max: ptr(5)}, 4, Not(HaveOccurred())),
		Entry("may only be odd with zero value", proofing.IntegerConstraints{MayOnlyBeOddOrZero: ptr(true)}, 0, Not(HaveOccurred())),
		Entry("may only be odd with even value", proofing.IntegerConstraints{MayOnlyBeOddOrZero: ptr(true)}, 4, MatchError(ContainSubstring("odd"))),
		Entry("may only be odd with odd value", proofing.IntegerConstraints{MayOnlyBeOddOrZero: ptr(true)}, 3, Not(HaveOccurred())),
		Entry("may only increase", proofing.IntegerConstraints{MayOnlyBeOddOrZero: ptr(true)}, 3, Not(HaveOccurred())),
		Entry("modulo with square number value", proofing.IntegerConstraints{Modulo: ptr(5)}, 25, Not(HaveOccurred())),
		Entry("modulo with less value", proofing.IntegerConstraints{Modulo: ptr(5)}, 4, MatchError(ContainSubstring("modulo"))),
		Entry("zero or min with zero value", proofing.IntegerConstraints{ZeroOrMin: ptr(5)}, 0, Not(HaveOccurred())),
		Entry("zero or min with less than min value", proofing.IntegerConstraints{ZeroOrMin: ptr(5)}, 3, MatchError(ContainSubstring("at least"))),
		Entry("zero or min with greater than min value", proofing.IntegerConstraints{ZeroOrMin: ptr(5)}, 1000, Not(HaveOccurred())),
		Entry("power of two and value 3", proofing.IntegerConstraints{PowerOfTwo: ptr(true)}, 3, MatchError(ContainSubstring("power of two"))),
		Entry("power of two and value 8", proofing.IntegerConstraints{PowerOfTwo: ptr(true)}, 8, Not(HaveOccurred())),
		Entry("power of two and value 0", proofing.IntegerConstraints{PowerOfTwo: ptr(true)}, 0, Not(HaveOccurred())),
	)
})

func ptr[T any](v T) *T {
	return &v
}

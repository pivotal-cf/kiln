package fetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/fetcher"
)

var _ = Describe("Contains", func() {

	// Is this really a case? :
	// Multiple matches: if set contains BOTH compiled and built, returns compiled.

	It("returns the matched element and true when element is an exact match in the provided set", func() {
		element := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		compiledReleaseSet := fetcher.CompiledReleaseSet{fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}: "foo"}

		got, result := compiledReleaseSet.Contains(element)
		Expect(got).To(Equal(element))
		Expect(result).To(BeTrue())
	})

	It("returns false when set is empty", func() {
		element := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		compiledReleaseSet := fetcher.CompiledReleaseSet{}

		_, result := compiledReleaseSet.Contains(element)
		Expect(result).To(BeFalse())
		_, result = compiledReleaseSet.Contains(fetcher.CompiledRelease{})
		Expect(result).To(BeFalse())
	})

	It("returns matched element and true when set only has partial match", func() {
		element := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "",
			StemcellVersion: "",
		}
		compiledReleaseSet := fetcher.CompiledReleaseSet{fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}: "foo"}

		got, result := compiledReleaseSet.Contains(element)

		Expect(got).To(Equal(fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}))
		Expect(result).To(BeTrue())
	})

	It("when desired release has non-empty stemcell info, "+
		"even if name and version matches an element in the set, returns false", func() {
		element := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "3.0.0",
		}
		compiledReleaseSet := fetcher.CompiledReleaseSet{fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}: "foo"}

		_, result := compiledReleaseSet.Contains(element)
		Expect(result).To(BeFalse())
	})
})

var _ = Describe("TransferElements", func() {
	// Cannot transfer elements from an empty set, return same source and dest sets as provided
	// Cannot transfer elements which do not exist , return same source and dest sets as provided
	// Can transfer all elements in a single set to the other, return
	// Can transfer subset of elements from source to dest, returns properly updated sets
	//
	// sourceSet.TransferElements(elementsToTransfer, destinationSet)
	It("returns unaltered sets when trying to transfer elements from an empty set", func() {
		element0 := fetcher.CompiledRelease{}
		element1 := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			Name:            "Some-release",
			Version:         "2.2.2",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}

		set1 := fetcher.CompiledReleaseSet{}
		set2 := fetcher.CompiledReleaseSet{element1: "foo"}
		transferSet := fetcher.CompiledReleaseSet{element2: "bar"}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source.Contains(element0)
		Expect(ok).To(BeFalse())
		_, ok = source.Contains(element1)
		Expect(ok).To(BeFalse())

		_, ok = dest.Contains(element2)
		Expect(ok).To(BeFalse())
		_, ok = dest.Contains(element1)
		Expect(ok).To(BeTrue())

	})

	It("returns unaltered sets when trying to transfer elements which do not exist in a populated source", func() {
		element1 := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			Name:            "Some-release",
			Version:         "2.2.2",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element3 := fetcher.CompiledRelease{
			Name:            "sum-release",
			Version:         "2.2.3",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}

		set1 := fetcher.CompiledReleaseSet{element1: "poo"}
		set2 := fetcher.CompiledReleaseSet{element2: "foo"}
		transferSet := fetcher.CompiledReleaseSet{element3: "bar"}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source.Contains(element1)
		Expect(ok).To(BeTrue())
		_, ok = source.Contains(element3)
		Expect(ok).To(BeFalse())

		_, ok = dest.Contains(element2)
		Expect(ok).To(BeTrue())
		_, ok = dest.Contains(element1)
		Expect(ok).To(BeFalse())
		_, ok = dest.Contains(element3)
		Expect(ok).To(BeFalse())
	})

	It("returns sets showing that the elements to transfer have in fact been moved", func() {
		element1 := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			Name:            "Some-release",
			Version:         "2.2.2",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element3 := element1

		set1 := fetcher.CompiledReleaseSet{element1: "poo"}
		set2 := fetcher.CompiledReleaseSet{element2: "foo"}
		transferSet := fetcher.CompiledReleaseSet{element3: "poo"}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source.Contains(element1)
		Expect(ok).To(BeFalse())
		_, ok = source.Contains(element3)
		Expect(ok).To(BeFalse())

		_, ok = dest.Contains(element2)
		Expect(ok).To(BeTrue())
		_, ok = dest.Contains(element3)
		Expect(ok).To(BeTrue())

		Expect(len(source)).To(Equal(0))

	})
	It("returns sets showing that the elements to transfer have in fact been "+
		"moved when not all elements from source are being transferred dest", func() {
		element1 := fetcher.CompiledRelease{
			Name:            "💩",
			Version:         "1.1.1",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			Name:            "Some-release",
			Version:         "2.2.2",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element3 := fetcher.CompiledRelease{
			Name:            "sum-release",
			Version:         "1.1.2",
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
		}
		element4 := element1

		set1 := fetcher.CompiledReleaseSet{element1: "poo", element2: "poop"}
		set2 := fetcher.CompiledReleaseSet{element3: "foo"}
		transferSet := fetcher.CompiledReleaseSet{element4: "poo"}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source.Contains(element1)
		Expect(ok).To(BeFalse())
		_, ok = source.Contains(element4)
		Expect(ok).To(BeFalse())
		_, ok = source.Contains(element2)
		Expect(ok).To(BeTrue())

		_, ok = dest.Contains(element1)
		Expect(ok).To(BeTrue())
		_, ok = dest.Contains(element3)
		Expect(ok).To(BeTrue())

		Expect(len(source)).ToNot(Equal(0))

	})

})

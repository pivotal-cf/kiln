package fetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/fetcher"
)

var _ = Describe("TransferElements", func() {
	// Cannot transfer elements from an empty set, return same source and dest sets as provided
	// Cannot transfer elements which do not exist , return same source and dest sets as provided
	// Can transfer all elements in a single set to the other, return
	// Can transfer subset of elements from source to dest, returns properly updated sets
	//
	// sourceSet.TransferElements(elementsToTransfer, destinationSet)
	It("returns unaltered sets when trying to transfer elements from an empty set", func() {
		id0 := fetcher.ReleaseID{}
		id1 := fetcher.ReleaseID{
			Name:    "💩",
			Version: "1.1.1",
		}
		element1 := fetcher.CompiledRelease{
			ID:              id1,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "foo",
		}
		id2 := fetcher.ReleaseID{
			Name:    "Some-release",
			Version: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			ID:              id2,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "bar",
		}

		set1 := fetcher.ReleaseSet{}
		set2 := fetcher.ReleaseSet{id1: element1}
		transferSet := fetcher.ReleaseSet{id2: element2}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source[id0]
		Expect(ok).To(BeFalse())
		_, ok = source[id1]
		Expect(ok).To(BeFalse())

		_, ok = dest[id2]
		Expect(ok).To(BeFalse())
		_, ok = dest[id1]
		Expect(ok).To(BeTrue())

	})

	It("returns unaltered sets when trying to transfer elements which do not exist in a populated source", func() {
		id1 := fetcher.ReleaseID{
			Name:    "💩",
			Version: "1.1.1",
		}
		element1 := fetcher.CompiledRelease{
			ID:              id1,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "poo",
		}
		id2 := fetcher.ReleaseID{
			Name:    "Some-release",
			Version: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			ID:              id2,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "foo",
		}
		id3 := fetcher.ReleaseID{
			Name:    "sum-release",
			Version: "2.2.3",
		}
		element3 := fetcher.CompiledRelease{
			ID:              id3,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "bar",
		}

		set1 := fetcher.ReleaseSet{id1: element1}
		set2 := fetcher.ReleaseSet{id2: element2}
		transferSet := fetcher.ReleaseSet{id3: element3}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source[id1]
		Expect(ok).To(BeTrue())
		_, ok = source[id3]
		Expect(ok).To(BeFalse())

		_, ok = dest[id2]
		Expect(ok).To(BeTrue())
		_, ok = dest[id1]
		Expect(ok).To(BeFalse())
		_, ok = dest[id3]
		Expect(ok).To(BeFalse())
	})

	It("returns sets showing that the elements to transfer have in fact been moved", func() {
		id1 := fetcher.ReleaseID{

			Name:    "💩",
			Version: "1.1.1",
		}
		element1 := fetcher.CompiledRelease{
			ID:              id1,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "poo",
		}
		id2 := fetcher.ReleaseID{

			Name:    "Some-release",
			Version: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			ID:              id2,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "foo",
		}
		id3 := id1
		element3 := element1

		set1 := fetcher.ReleaseSet{id1: element1}
		set2 := fetcher.ReleaseSet{id2: element2}
		transferSet := fetcher.ReleaseSet{id3: element3}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source[id1]
		Expect(ok).To(BeFalse())
		_, ok = source[id3]
		Expect(ok).To(BeFalse())

		_, ok = dest[id2]
		Expect(ok).To(BeTrue())
		_, ok = dest[id3]
		Expect(ok).To(BeTrue())

		Expect(len(source)).To(Equal(0))

	})
	It("returns sets showing that the elements to transfer have in fact been "+
		"moved when not all elements from source are being transferred dest", func() {
		id1 := fetcher.ReleaseID{
			Name:    "💩",
			Version: "1.1.1",
		}
		element1 := fetcher.CompiledRelease{
			ID:              id1,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "poo",
		}
		id2 := fetcher.ReleaseID{
			Name:    "Some-release",
			Version: "2.2.2",
		}
		element2 := fetcher.CompiledRelease{
			ID:              id2,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "poop",
		}
		id3 := fetcher.ReleaseID{
			Name:    "sum-release",
			Version: "1.1.2",
		}
		element3 := fetcher.CompiledRelease{
			ID:              id3,
			StemcellOS:      "an-os",
			StemcellVersion: "2.2.2",
			Path:            "foo",
		}
		id4 := id1
		element4 := element1

		set1 := fetcher.ReleaseSet{id1: element1, id2: element2}
		set2 := fetcher.ReleaseSet{id3: element3}
		transferSet := fetcher.ReleaseSet{id4: element4}

		source, dest := set1.TransferElements(transferSet, set2)

		_, ok := source[id1]
		Expect(ok).To(BeFalse())
		_, ok = source[id4]
		Expect(ok).To(BeFalse())
		_, ok = source[id2]
		Expect(ok).To(BeTrue())

		_, ok = dest[id1]
		Expect(ok).To(BeTrue())
		_, ok = dest[id3]
		Expect(ok).To(BeTrue())

		Expect(len(source)).ToNot(Equal(0))

	})

})

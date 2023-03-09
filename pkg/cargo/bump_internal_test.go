package cargo

import (
	"testing"

	"github.com/google/go-github/v40/github"
	. "github.com/onsi/gomega"
)

func TestInternal_deduplicateReleasesWithTheSameTagName(t *testing.T) {
	please := NewWithT(t)
	b := Bump{
		Releases: []*github.RepositoryRelease{
			{TagName: ptr("Y")},
			{TagName: ptr("1")},
			{TagName: ptr("2")},
			{TagName: ptr("3")},
			{TagName: ptr("3")},
			{TagName: ptr("3")},
			{TagName: ptr("X")},
			{TagName: ptr("2")},
			{TagName: ptr("4")},
			{TagName: ptr("4")},
		},
	}
	b = deduplicateReleasesWithTheSameTagName(b)
	tags := make([]string, 0, len(b.Releases))
	for _, r := range b.Releases {
		tags = append(tags, r.GetTagName())
	}
	please.Expect(tags).To(Equal([]string{
		"Y",
		"1",
		"2",
		"3",
		"X",
		"4",
	}))
}

func ptr[T any](v T) *T { return &v }

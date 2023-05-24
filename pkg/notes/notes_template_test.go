package notes

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v40/github"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
)

func Test_defaultReleaseNotesTemplate(t *testing.T) {
	t.Run("empty github release body", func(t *testing.T) {
		please := NewWithT(t)
		tmp, err := DefaultTemplateFunctions(template.New("")).Parse(DefaultTemplate())
		please.Expect(err).NotTo(HaveOccurred())
		var b bytes.Buffer
		err = tmp.Execute(&b, Data{
			Version: semver.MustParse("0.0.0"),
			Components: []BOSHReleaseData{
				{
					ComponentLock: component.Lock{Name: "banana", Version: "1.2"},
					Releases: []*github.RepositoryRelease{
						{TagName: strPtr("1.1"), Body: strPtr("\n   ")},
						{TagName: strPtr("1.2"), Body: strPtr("")},
					},
				},
			},
		})
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(b.String()).To(ContainSubstring("<tr><td>banana</td><td>1.2</td><td></td></tr>"))
	})
}

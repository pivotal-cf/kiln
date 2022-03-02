package release

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v40/github"
	Ω "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/pkg/component"
)

func Test_defaultReleaseNotesTemplate(t *testing.T) {
	t.Run("empty github release body", func(t *testing.T) {
		please := Ω.NewWithT(t)
		tmp, err := DefaultTemplateFuncs(template.New("")).Parse(DefaultNotesTemplate())
		please.Expect(err).NotTo(Ω.HaveOccurred())
		var b bytes.Buffer
		err = tmp.Execute(&b, NotesData{
			Version: semver.MustParse("0.0"),
			Components: []ComponentData{
				{
					Lock: component.Lock{Name: "banana", Version: "1.2"},
					Releases: []*github.RepositoryRelease{
						{TagName: strPtr("1.1"), Body: strPtr("\n   ")},
						{TagName: strPtr("1.2"), Body: strPtr("")},
					},
				},
			},
		})
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(b.String()).To(Ω.ContainSubstring("<tr><td>banana</td><td>1.2</td><td></td></tr>"))
	})
}

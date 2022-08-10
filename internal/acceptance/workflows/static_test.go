package workflows

import (
	"embed"
	"io/fs"
	"regexp"
	"strings"
	"testing"
)

//go:embed *.feature
var featuresFiles embed.FS

func TestFeatures(t *testing.T) {
	asA := regexp.MustCompile(`(?mi)feature:.*(as a).*`)

	walkErr := fs.WalkDir(featuresFiles, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.TrimSuffix(d.Name(), ".feature")

		t.Run(name, func(t *testing.T) {
			featureFile, err := featuresFiles.ReadFile(path)
			if err != nil {
				t.Error(t)
				return
			}

			if !asA.Match(featureFile) {
				t.Error(`features must define the primary user (robot, developer...) of the feature with an "as a" in the feature name`)
			}
		})

		return nil
	})
	if walkErr != nil {
		t.Error(walkErr)
	}
}

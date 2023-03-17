package commands_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/component"
	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

func TestOSM_Execute(t *testing.T) {
	t.Run("it outputs an OSM File", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.ComponentSpec{
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/cloudfoundry/banana",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.ComponentLock{
				{Name: "banana", Version: "1.2.3"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionReturns(component.Lock{}, nil)

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)
		err := cmd.Execute([]string{"--no-download", "--kilnfile", kfp})

		Expect(err).ToNot(HaveOccurred())

		Eventually(outBuffer).Should(gbytes.Say("other:banana:1.2.3:"), "output should contain special formatted row")
		Eventually(outBuffer).Should(gbytes.Say("  name: banana"))
		Eventually(outBuffer).Should(gbytes.Say("  version: 1.2.3"))
		Eventually(outBuffer).Should(gbytes.Say("  repository: Other"))
		Eventually(outBuffer).Should(gbytes.Say("  url: https://github.com/cloudfoundry/banana"))
		Eventually(outBuffer).Should(gbytes.Say("  license: Apache2.0"))
		Eventually(outBuffer).Should(gbytes.Say("  interactions:"))
		Eventually(outBuffer).Should(gbytes.Say("  - Distributed - Calling Existing Classes"))
		Eventually(outBuffer).Should(gbytes.Say("  other-distribution: ./banana-1.2.3.zip"))
	})

	t.Run("it excludes non-cloudfoundry entries", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.ComponentSpec{
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/cloudfoundry/banana",
				},
				{
					Name:             "apple",
					GitHubRepository: "https://github.com/pivotal/apple",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.ComponentLock{
				{Name: "banana", Version: "1.2.3"},
				{Name: "apple", Version: "1.2.4"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionCalls(func(spec cargo.ComponentSpec, _ bool) (cargo.ComponentLock, error) {
			switch spec.Name {
			case "banana":
				return component.Lock{}, nil
			}
			return component.Lock{}, component.ErrNotFound
		})

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)

		Expect(cmd.Execute([]string{"--no-download", "--kilnfile", kfp})).Error().ToNot(HaveOccurred())

		Consistently(outBuffer).ShouldNot(gbytes.Say("other:apple:1.2.4:"), "output should omit entry for non-cf repos")
		Eventually(outBuffer).Should(gbytes.Say("other:banana:1.2.3:"), "output should contain cf entry")
	})

	t.Run("it finds buildpacks if they contain \"-offline\"", func(t *testing.T) {
		RegisterTestingT(t)

		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.ComponentSpec{
				{
					Name: "lemon-offline-buildpack",
				},
				{
					Name:             "banana",
					GitHubRepository: "https://github.com/pivotal/banana",
				},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.ComponentLock{
				{Name: "lemon-offline-buildpack", Version: "1.2.3"},
				{Name: "banana", Version: "1.2.3"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		rs := new(fakes.ReleaseStorage)
		rs.FindReleaseVersionCalls(func(spec cargo.ComponentSpec, _ bool) (cargo.ComponentLock, error) {
			switch spec.Name {
			case "lemon-buildpack":
				return component.Lock{
					RemotePath: "https://bosh.io/d/github.com/cloudfoundry/lemon-buildpack-release?v=1.2.3",
				}, nil
			case "banana":
				return component.Lock{}, nil
			}
			return component.Lock{}, component.ErrNotFound
		})

		outBuffer := gbytes.NewBuffer()
		defer outBuffer.Close()

		logger := log.New(outBuffer, "", 0)
		cmd := commands.NewOSM(logger, rs)

		Expect(cmd.Execute([]string{"--no-download", "--kilnfile", kfp})).Error().ToNot(HaveOccurred())

		Eventually(outBuffer).Should(gbytes.Say("other:lemon-offline-buildpack:1.2.3:"), "output should contain special formatted row")
		Eventually(outBuffer).Should(gbytes.Say("  name: lemon-offline-buildpack"))
		Eventually(outBuffer).Should(gbytes.Say("  version: 1.2.3"))
		Eventually(outBuffer).Should(gbytes.Say("  repository: Other"))
		Eventually(outBuffer).Should(gbytes.Say("  url: https://github.com/cloudfoundry/lemon-buildpack-release"))
		Eventually(outBuffer).Should(gbytes.Say("  license: Apache2.0"))
		Eventually(outBuffer).Should(gbytes.Say("  interactions:"))
		Eventually(outBuffer).Should(gbytes.Say("  - Distributed - Calling Existing Classes"))
		Eventually(outBuffer).Should(gbytes.Say("  other-distribution: ./lemon-offline-buildpack-1.2.3.zip"))
	})
}

func writeYAML(t *testing.T, path string, data interface{}) {
	t.Helper()
	buf, err := yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeAndIgnoreError(f)

	_, err = f.Write(buf)
	if err != nil {
		t.Fatal(err)
	}
}

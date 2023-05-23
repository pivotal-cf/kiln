package commands

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v2"
)

const invalidYAML = `}`

func TestGlaze_Execute(t *testing.T) {
	t.Run("it updates the Kilnfile", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseSpec{
				{Name: "banana"},
				{Name: "orange", Version: "~ 8.0"},
			},
			Stemcell: cargo.Stemcell{
				OS: "alpine",
			},
		})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseLock{
				{Name: "banana", Version: "1.2.3"},
				{Name: "orange", Version: "8.0.8"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		})

		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).ToNot(HaveOccurred())

		var updatedKilnfile cargo.Kilnfile
		readYAML(t, kfp, &updatedKilnfile)

		g.Expect(updatedKilnfile).To(Equal(cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseSpec{
				{Name: "banana", Version: "1.2.3"},
				{Name: "orange", Version: "8.0.8"},
			},
			Stemcell: cargo.Stemcell{
				OS:      "alpine",
				Version: "42.0",
			},
		}))
	})

	t.Run("Kilnfile passed in as argument", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Kilnfile is missing", func(t *testing.T) {
		tmp := t.TempDir()
		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", tmp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring("Kilnfile")))
	})

	t.Run("Kilnfile path is a directory", func(t *testing.T) {
		tmp := t.TempDir()

		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})
		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", tmp})

		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("Kilnfile lock is missing", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring("Kilnfile.lock")))
	})

	t.Run("Kilnfile has a release not in Kilnfile.lock", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseSpec{
				{Name: "banana"},
			},
		})
		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseLock{},
		})
		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring(`"banana" not found in Kilnfile.lock`)))
	})

	t.Run("Kilnfile has a release not in Kilnfile.lock", func(t *testing.T) {
		tmp := t.TempDir()

		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{
			Releases: []cargo.BOSHReleaseSpec{
				{Name: "banana"},
			},
		})
		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{
			Releases: []cargo.BOSHReleaseLock{},
		})
		cmd := new(Glaze)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring(`"banana" not found in Kilnfile.lock`)))
	})

	t.Run("bad flag passed", func(t *testing.T) {
		cmd := new(Glaze)
		err := cmd.Execute([]string{"--unknown-flag"})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring(`flag provided but not defined`)))
	})

	t.Run("bad yaml", func(t *testing.T) {
		t.Run("Kilnfile", func(t *testing.T) {
			tmp := t.TempDir()
			kfp := filepath.Join(tmp, "Kilnfile")
			_ = os.WriteFile(kfp, []byte(invalidYAML), 0o777)
			cmd := new(Glaze)
			err := cmd.Execute([]string{"--kilnfile", kfp})
			g := NewWithT(t)
			g.Expect(err).To(MatchError(And(
				ContainSubstring(`Kilnfile`),
				ContainSubstring(`yaml:`),
			)))
		})
		t.Run("Kilnfile.lock", func(t *testing.T) {
			tmp := t.TempDir()
			kfp := filepath.Join(tmp, "Kilnfile")
			_ = os.WriteFile(kfp, []byte(`{}`), 0o777)
			klp := filepath.Join(tmp, "Kilnfile.lock")
			_ = os.WriteFile(klp, []byte(invalidYAML), 0o777)
			cmd := new(Glaze)
			err := cmd.Execute([]string{"--kilnfile", kfp})
			g := NewWithT(t)
			g.Expect(err).To(MatchError(And(
				ContainSubstring(`Kilnfile.lock`),
				ContainSubstring(`yaml:`),
			)))
		})
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

func readYAML(t *testing.T, path string, data interface{}) {
	t.Helper()

	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal()
	}

	err = yaml.Unmarshal(buf, data)
	if err != nil {
		t.Fatal(err)
	}
}

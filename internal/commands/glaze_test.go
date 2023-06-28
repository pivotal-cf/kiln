package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v2"
)

const invalidYAML = `}`

type glazeWithFakeImplementation struct {
	Glaze
	GlazeFuncCalled, DeGlazeFuncCalled bool
	glazeFuncError, deGlazeFuncError   error
}

func (fake *glazeWithFakeImplementation) glaze(*cargo.Kilnfile, cargo.KilnfileLock) error {
	fake.GlazeFuncCalled = true
	return fake.glazeFuncError
}

func (fake *glazeWithFakeImplementation) deGlaze(*cargo.Kilnfile, cargo.KilnfileLock) error {
	fake.DeGlazeFuncCalled = true
	return fake.deGlazeFuncError
}

func newGlazeWithFake(glazeFuncError, deGlazeFuncError error) *glazeWithFakeImplementation {
	result := &glazeWithFakeImplementation{
		glazeFuncError:   glazeFuncError,
		deGlazeFuncError: deGlazeFuncError,
	}
	result.Glaze.glaze = result.glaze
	result.Glaze.deGlaze = result.deGlaze
	return result
}

func TestGlaze_Execute(t *testing.T) {
	t.Run("Kilnfile passed in as argument", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).ToNot(HaveOccurred())
	})

	t.Run("Kilnfile is missing", func(t *testing.T) {
		tmp := t.TempDir()
		err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", tmp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring("Kilnfile")))
	})

	t.Run("Kilnfile path is a directory", func(t *testing.T) {
		tmp := t.TempDir()

		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})
		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", tmp})

		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("Kilnfile lock is missing", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring("Kilnfile.lock")))
	})

	t.Run("bad flag passed", func(t *testing.T) {
		err := newGlazeWithFake(nil, nil).Execute([]string{"--unknown-flag"})

		g := NewWithT(t)
		g.Expect(err).To(MatchError(ContainSubstring(`flag provided but not defined`)))
	})

	t.Run("bad yaml", func(t *testing.T) {
		t.Run("Kilnfile", func(t *testing.T) {
			tmp := t.TempDir()
			kfp := filepath.Join(tmp, "Kilnfile")
			_ = os.WriteFile(kfp, []byte(invalidYAML), 0o777)
			err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", kfp})
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
			err := newGlazeWithFake(nil, nil).Execute([]string{"--kilnfile", kfp})
			g := NewWithT(t)
			g.Expect(err).To(MatchError(And(
				ContainSubstring(`Kilnfile.lock`),
				ContainSubstring(`yaml:`),
			)))
		})
	})

	t.Run("when undo is passed", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := newGlazeWithFake(nil, nil)
		err := cmd.Execute([]string{"--undo", "--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cmd.DeGlazeFuncCalled).To(BeTrue(), "it calls deGlaze")
	})

	t.Run("when de-glaze fails", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := newGlazeWithFake(nil, fmt.Errorf("banana"))
		err := cmd.Execute([]string{"--undo"})

		g := NewWithT(t)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("when undo is not passed", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := newGlazeWithFake(nil, nil)
		err := cmd.Execute([]string{"--kilnfile", kfp})

		g := NewWithT(t)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(cmd.GlazeFuncCalled).To(BeTrue(), "it calls glaze")
	})

	t.Run("when glaze fails", func(t *testing.T) {
		tmp := t.TempDir()
		kfp := filepath.Join(tmp, "Kilnfile")
		writeYAML(t, kfp, cargo.Kilnfile{})

		klp := filepath.Join(tmp, "Kilnfile.lock")
		writeYAML(t, klp, cargo.KilnfileLock{})

		cmd := newGlazeWithFake(nil, fmt.Errorf("banana"))
		err := cmd.Execute([]string{})

		g := NewWithT(t)
		g.Expect(err).To(HaveOccurred())
	})
}

func writeYAML(t *testing.T, path string, data any) {
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

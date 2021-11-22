package commands_test

import (
	"testing"

	Ω "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

func TestCommand_Execute(t *testing.T) {
	t.Run("with a standard flag", func(t *testing.T) {
		please := Ω.NewWithT(t)

		s := new(fakes.KilnfileStorer)
		s.LoadReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, nil)

		var w CommandUsingKilnfile
		w.E = func(args []string, parseFunc commands.OptionsParseFunc) error {
			_, _, _, _ = parseFunc(args, &w.Options)
			return nil
		}

		err := commands.Kiln{
			Wrapped:       &w,
			KilnfileStore: s,
		}.Execute([]string{"--kilnfile=some-path"})

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(w.Options.Kilnfile).To(Ω.Equal("some-path"))
	})

	t.Run("with a standard flag", func(t *testing.T) {
		please := Ω.NewWithT(t)

		s := new(fakes.KilnfileStorer)
		s.LoadReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, nil)

		var w CommandUpdatingKilnfileLock
		w.E = func(args []string, parseFunc commands.OptionsParseFunc) (cargo.KilnfileLock, error) {
			_, _, _, _ = parseFunc(args, &w.Options)
			return cargo.KilnfileLock{}, nil
		}

		err := commands.Kiln{
			Wrapped:       &w,
			KilnfileStore: s,
		}.Execute([]string{"--kilnfile=some-path"})

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(w.Options.Kilnfile).To(Ω.Equal("some-path"))
	})

	t.Run("with a non-standard flags", func(t *testing.T) {
		please := Ω.NewWithT(t)

		s := new(fakes.KilnfileStorer)
		s.LoadReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, nil)

		var w CommandWithMoreFlags
		w.E = func(args []string, parseFunc commands.OptionsParseFunc) error {
			_, _, _, _ = parseFunc(args, &w.Options)
			return nil
		}

		err := commands.Kiln{
			Wrapped:       &w,
			KilnfileStore: s,
		}.Execute([]string{"--kilnfile=some-path", "--other=config"})

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(w.Options.Other).To(Ω.Equal("config"))
	})

	t.Run("with variables in the Kilnfile", func(t *testing.T) {
		please := Ω.NewWithT(t)

		fs := memfs.New()

		please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{
			Slug: "example",
		})).NotTo(Ω.HaveOccurred())
		please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{})).NotTo(Ω.HaveOccurred())

		s := commands.KilnfileStore{
			FS: fs,
		}

		var (
			kf cargo.Kilnfile
			w  CommandWithMoreFlags
		)
		w.E = func(args []string, parseFunc commands.OptionsParseFunc) error {
			var err error
			kf, _, _, err = parseFunc(args, &w.Options)
			return err
		}

		err := commands.Kiln{
			Wrapped:       &w,
			KilnfileStore: s,
			StatFn:        fs.Stat,
		}.Execute([]string{})

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(kf.Slug).To(Ω.Equal("example"))
	})
}

const (
	kilnfileWithKey = `---
release_sources:
- access_key_id: "some-key"
`
	kilnfileWithVar = `---
release_sources:
- access_key_id: $(variable "some-key")
`
)

func TestKilnfileStore_Load(t *testing.T) {
	setup := func(t *testing.T, kilnfileContent string, vars map[string]interface{}) (options.Standard, commands.KilnfileStore) {
		t.Helper()

		fs := memfs.New()
		vs := new(fakes.VariablesService)
		if vars == nil {
			vars = make(map[string]interface{})
		}
		vs.FromPathsAndPairsReturns(vars, nil)
		sf := options.Standard{
			Kilnfile: "Kilnfile",
		}
		s := commands.KilnfileStore{FS: fs, VS: vs}
		{
			f, _ := fs.Create(sf.Kilnfile)
			_, _ = f.Write([]byte(kilnfileContent))
			_ = f.Close()
		}
		{
			f, _ := fs.Create(sf.KilnfileLockPath())
			_, _ = f.Write(nil)
			_ = f.Close()
		}
		return sf, s
	}

	t.Run("no interpolation required", func(t *testing.T) {
		please := Ω.NewWithT(t)

		sf, sv := setup(t, kilnfileWithKey, nil)
		_, _, err := sv.Load(sf)
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("variable from flag/file", func(t *testing.T) {
		please := Ω.NewWithT(t)

		sf, sv := setup(t, kilnfileWithVar, map[string]interface{}{
			"some-key": "key",
		})

		kf, _, err := sv.Load(sf)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(kf.ReleaseSources).To(Ω.HaveLen(1))
		please.Expect(kf.ReleaseSources[0].AccessKeyId).To(Ω.Equal("key"))
	})
}

type CommandWithMoreFlags struct {
	Options struct {
		options.Standard

		Other string `long:"other"`
	}

	E func([]string, commands.OptionsParseFunc) error
}

func (c CommandWithMoreFlags) Usage() jhanda.Usage { return jhanda.Usage{} }
func (c CommandWithMoreFlags) KilnExecute(args []string, fn commands.OptionsParseFunc) error {
	return c.E(args, fn)
}

type CommandUsingKilnfile struct {
	Options struct {
		options.Standard
	}

	E func([]string, commands.OptionsParseFunc) error
}

func (c CommandUsingKilnfile) Usage() jhanda.Usage { return jhanda.Usage{} }
func (c CommandUsingKilnfile) KilnExecute(args []string, fn commands.OptionsParseFunc) error {
	return c.E(args, fn)
}

type CommandUpdatingKilnfileLock struct {
	Options struct {
		options.Standard
	}

	E func([]string, commands.OptionsParseFunc) (cargo.KilnfileLock, error)
}

func (c CommandUpdatingKilnfileLock) Usage() jhanda.Usage { return jhanda.Usage{} }
func (c CommandUpdatingKilnfileLock) KilnExecute(args []string, fn commands.OptionsParseFunc) (cargo.KilnfileLock, error) {
	return c.E(args, fn)
}

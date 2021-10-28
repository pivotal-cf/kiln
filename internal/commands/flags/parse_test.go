package flags_test

import (
	"github.com/go-git/go-billy/v5"
	"io"
	"os/exec"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
)

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

func TestLoadKilnfiles(t *testing.T) {
	type fixtures struct {
		fs billy.Basic
		vs *fakes.VariablesService
		runSpy flags.RunFunc
	}
	setup := func(t *testing.T, kilnfileContent string, vars map[string]interface{}, runStub flags.RunFunc) (flags.Standard, fixtures) {
		t.Helper()

		fs := memfs.New()
		vs := new(fakes.VariablesService)
		if vars == nil {
			vars = make(map[string]interface{})
		}
		vs.FromPathsAndPairsReturns(vars, nil)
		s := flags.Standard{
			Kilnfile: "Kilnfile",
		}
		fx := fixtures{
			fs: fs,
			vs: vs,
			runSpy: func(stdOut io.Writer, cmd *exec.Cmd) error {
				if runStub == nil{
					return nil
				}
				return runStub(stdOut, cmd)
			},
		}
		{
			f, _ := fs.Create(s.Kilnfile)
			_, _ = f.Write([]byte(kilnfileContent))
			_ = f.Close()
		}
		{
			f, _ := fs.Create(s.KilnfileLockPath())
			_, _ = f.Write(nil)
			_ = f.Close()
		}
		return s, fx
	}

	t.Run("no interpolation required", func(t *testing.T) {
		please := Ω.NewWithT(t)

		s, fx := setup(t, kilnfileWithKey, nil, nil)
		_, _, err := s.LoadKilnfiles(fx.fs, fx.vs, fx.runSpy)
		please.Expect(err).NotTo(Ω.HaveOccurred())
	})

	t.Run("variable from flag/file", func(t *testing.T) {
		please := Ω.NewWithT(t)

		s, tCtx := setup(t, kilnfileWithVar, map[string]interface{}{
			"some-key": "key",
		}, nil)
		kf, _, err := s.LoadKilnfiles(tCtx.fs, tCtx.vs, tCtx.runSpy)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(kf.ReleaseSources).To(Ω.HaveLen(1))
		please.Expect(kf.ReleaseSources[0].AccessKeyId).To(Ω.Equal("key"))
	})

	t.Run("with variable from credential hook", func(t *testing.T) {
		t.SkipNow()
		please := Ω.NewWithT(t)

		var (
			// commands []*exec.Cmd
			runCallCount = 0
		)
		s, fx := setup(t, kilnfileWithVar, nil, func(w io.Writer, cmd *exec.Cmd) error {
			//_, _ = w.Write([]byte(`"some-key": "key"`))
			//commands = append(commands, cmd)
			runCallCount++
			return nil
		})
		kf, _, err := s.LoadKilnfiles(fx.fs, fx.vs, fx.runSpy)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(kf.ReleaseSources).To(Ω.HaveLen(1))
		please.Expect(kf.ReleaseSources[0].AccessKeyId).To(Ω.Equal("key"))
		please.Expect(runCallCount).To(Ω.Equal(1))
	})
}

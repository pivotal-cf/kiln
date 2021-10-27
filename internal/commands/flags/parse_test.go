package flags_test

import (
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
)

func TestLoadKilnfiles(t *testing.T) {
	please := Ω.NewWithT(t)
	s := flags.Standard{
		Kilnfile: "Kilnfile",
	}

	fs := memfs.New()
	vs := new(fakes.VariablesService)

	{
		f, _ := fs.Create("Kilnfile")
		_, _ = f.Write(nil)
		_ = f.Close()
	}
	{
		f, _ := fs.Create("Kilnfile.lock")
		_, _ = f.Write(nil)
		_ = f.Close()
	}

	_, _, err := s.LoadKilnfiles(fs, vs)
	please.Expect(err).NotTo(Ω.HaveOccurred())
}

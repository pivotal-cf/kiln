package commands_test

import (
	"bytes"
	"io/ioutil"
	"testing"

	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ jhanda.Command = commands.ReleaseNotes{}

func TestReleaseNotes_Usage(t *testing.T) {
	please := Ω.NewWithT(t)

	rn := commands.ReleaseNotes{}

	please.Expect(rn.Usage().Description).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().ShortDescription).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().Flags).NotTo(Ω.BeNil())
}

func TestReleaseNotes_Execute(t *testing.T) {
	please := Ω.NewWithT(t)

	t.Setenv("GITHUB_TOKEN", "")

	repo, _ := git.Init(memory.NewStorage(), memfs.New())

	revisionResolver := new(fakes.RevisionResolver)
	var (
		initialHash, finalHash plumbing.Hash
	)
	fill(initialHash[:], '1')
	fill(finalHash[:], '9')
	revisionResolver.ResolveRevisionReturnsOnCall(0, &initialHash, nil)
	revisionResolver.ResolveRevisionReturnsOnCall(1, &finalHash, nil)

	historicKilnfileLock := new(fakes.HistoricKilnfileLock)
	historicKilnfileLock.ReturnsOnCall(0, cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{Name: "banana", Version: "0.1.0"},
			{Name: "lemon", Version: "1.1.0"},
		},
	}, nil)
	historicKilnfileLock.ReturnsOnCall(1, cargo.KilnfileLock{
		Releases: []cargo.ComponentLock{
			{Name: "banana", Version: "0.1.0"},
			{Name: "lemon", Version: "1.2.0"},
		},
	}, nil)

	historicVersion := new(fakes.HistoricVersion)
	historicVersion.Returns("0.1.0-build.50000", nil)

	readFileCount := 0
	readFileFunc := func(fp string) ([]byte, error) {
		readFileCount++
		return nil, nil
	}

	var output bytes.Buffer
	rn := commands.ReleaseNotes{
		Repository:           repo,
		RevisionResolver:     revisionResolver,
		HistoricKilnfileLock: historicKilnfileLock.Spy,
		HistoricVersion:      historicVersion.Spy,
		Writer:               &output,
		ReadFile:             readFileFunc,
	}

	err := rn.Execute([]string{
		"--release-date=2021-11-05",
		"tile/1.1.0",
		"tile/1.2.0",
	})
	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(revisionResolver.ResolveRevisionCallCount()).To(Ω.Equal(2))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(0)).To(Ω.Equal(plumbing.Revision("tile/1.1.0")))
	please.Expect(revisionResolver.ResolveRevisionArgsForCall(1)).To(Ω.Equal(plumbing.Revision("tile/1.2.0")))

	please.Expect(historicVersion.CallCount()).To(Ω.Equal(1))
	_, historicVersionHashArg, _ := historicVersion.ArgsForCall(0)
	please.Expect(historicVersionHashArg).To(Ω.Equal(finalHash))

	please.Expect(readFileCount).To(Ω.Equal(0))
	expected, err := ioutil.ReadFile("testdata/release_notes_output.md")
	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(output.String()).To(Ω.Equal(string(expected)))
}

func fill(buf []byte, value byte) {
	for i := range buf {
		buf[i] = value
	}
}

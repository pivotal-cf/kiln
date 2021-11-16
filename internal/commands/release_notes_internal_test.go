package commands

import (
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	Ω "github.com/onsi/gomega"
	"testing"
)

func TestInternal_getGithubRemoteRepoOwnerAndName(t *testing.T) {
	t.Run("when there is a github http remote", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"https://github.com/pivotal-cf/kiln",
			},
		})
		o, r, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"))
		please.Expect(r).To(Ω.Equal("kiln"))
	})

	t.Run("when there is a github ssh remote", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		o, r, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"))
		please.Expect(r).To(Ω.Equal("kiln"))
	})

	t.Run("when there are no remotes", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).To(Ω.MatchError(Ω.ContainSubstring("not found")))
	})

	t.Run("when there are many remotes", func(t *testing.T) {
		please := Ω.NewWithT(t)

		repo, _ := git.Init(memory.NewStorage(), memfs.New())
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "fork",
			URLs: []string{
				"git@github.com:crhntr/kiln.git",
			},
		})
		_, _ = repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{
				"git@github.com:pivotal-cf/kiln.git",
			},
		})
		o, _, err := getGithubRemoteRepoOwnerAndName(repo)
		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(o).To(Ω.Equal("pivotal-cf"), "it uses the remote with name 'origin'")
	})
}

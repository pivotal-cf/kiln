package commands_test

import (
	_ "embed"
	"errors"
	"github.com/go-git/go-billy/v5/memfs"
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"testing"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ jhanda.Command = commands.Stemcell{}

func TestDownloadStemcell_Usage(t *testing.T) {
	please := Ω.NewWithT(t)

	s := commands.Stemcell{}

	please.Expect(s.Usage().Description).NotTo(Ω.BeEmpty())
	please.Expect(s.Usage().ShortDescription).NotTo(Ω.BeEmpty())
	please.Expect(s.Usage().Flags).NotTo(Ω.BeNil())
}

const testKilnfileLockLocation = "/Users/hallr/workspace/kiln/testKilnfile.lock"
const testKilnfileLock = `
Releases:
- name: routing
  sha1: e5d99ce00db1858a0bc18449780433514a3d0e13
  version: 0.227.0
  remote_source: compiled-Releases
  remote_path: by_stemcell/ubuntu-xenial/456.207/routing/routing-0.227.0-ubuntu-xenial-456.207.tgz
stemcell_criteria:
  os: ubuntu-xenial
  version: "456.207"
`
const testPivnetToken = "wtxh-VGyz-Pdh8YgYPH6"

func TestDownloadStemcell_Execute(t *testing.T) {
	t.Run("when we call the download stemcell command", func(t *testing.T) {
		t.Run("we expect the test stemcell criteria to match our testKilnfile.lock", func(t *testing.T) {
			please := Ω.NewWithT(t)
			fs := memfs.New()
			p := new(fakes.ProductFiles)
			r := new(fakes.ReleaseLister)
			e := new(fakes.EulaAccepter)
			s := commands.Stemcell{ProductFiles: p, Releases: r, Eula: e}
			s.FS = fs

			p.DownloadForReleaseReturns(errors.New("BANANA"))
			p.ListForReleaseReturns([]pivnet.ProductFile{{}}, nil)
			r.ListReturns(nil, errors.New("BANANA"))
			e.AcceptReturns(errors.New("BANANA"))

			please.Expect(fsWriteYAML(fs, "Kilnfile", cargo.Kilnfile{})).NotTo(Ω.HaveOccurred())
			please.Expect(fsWriteYAML(fs, "Kilnfile.lock", cargo.KilnfileLock{
				Releases: []cargo.ComponentLock{
					{
						Name:    "lemon",
						Version: "3.0.0",

						RemoteSource: "new-Releases",
						RemotePath:   "lemon-3.0.0",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "alpine",
					Version: "9.0.0",
				},
			})).NotTo(Ω.HaveOccurred())

			err := s.Execute(nil)

			slug := r.ListArgsForCall(0)
			please.Expect(slug).To(Ω.Equal("alpine"))

			please.Expect(err).NotTo(Ω.HaveOccurred())

			//input := d.DownloadArgsForCall(0)
			//
			//please.Expect(input.Version).To(Ω.Equal("9.0.0"))
		})

		/*
			testPivnetAccessTokenService := pivnet.NewAccessTokenOrLegacyToken(testPivnetToken, testPivnetClientConfig.Host, testPivnetClientConfig.SkipSSLValidation)
			testPivnetClient := pivnet.NewClient(testPivnetAccessTokenService, testPivnetClientConfig, testLogger)
			//Looking for an idea on what to verify in the testPivnetClient in this test, or if it's necessary

			t.Run("we expect the token service & pivnet Client info to be populated", func(t *testing.T) {
				please.Expect(testPivnetAccessTokenService.AccessToken()).To(Ω.Not(Ω.BeNil()))
				please.Expect(testPivnetClient.Releases).To(Ω.Not(Ω.BeNil()))
			})

			t.Run("we should be able to create a Stemcell Downloader, and download our stemcell", func(t *testing.T) {
				testStemcellDownloader := commands.StemcellClient{
					PivnetClient: testPivnetClient,
					Logger:       testStdoutLogger,
				}
				input := d.DownloadArgsForCall(0)
				testStemcellDownloaderInput := commands.DownloadStemcellInput{
					OS:        input.OS,
					Version:   input.Version,
					IaaS:      "vsphere",
					OutputDir: "/tmp",
				}
				err := testStemcellDownloader.Download(testStemcellDownloaderInput)
				please.Expect(err).To(Ω.BeNil())

			})

		*/
	})
}

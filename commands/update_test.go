package commands_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("Update", func() {
	var _ jhanda.Command = commands.Update{}
	When("Execute is called", func() {
		var (
			update                                                 *commands.Update
			tmpDir, someAssetsSpecFilePath, someAssetsLockFilePath string
			stemcellsVersionsService                               fakes.VersionsService
		)
		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "fetch-test")
			// fmt.Println(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			someAssetsSpecFilePath = filepath.Join(tmpDir, "assets.yml")
			someAssetsLockFilePath = filepath.Join(tmpDir, "assets.lock")
			Expect(
				ioutil.WriteFile(someAssetsSpecFilePath, []byte(initallAssetsYAMLFileContents), 0644),
			).NotTo(HaveOccurred())

			stemcellsVersionsService = fakes.VersionsService{}
			stemcellsVersionsService.Returns.Versions = []string{
				"3580.0",
				"3586.7",
				"3586.2",
				"3586.3",
				"3588.0",
				"3587.1",
			}
			update = &commands.Update{
				StemcellsVersionsService: &stemcellsVersionsService,
			}
		})
		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		When("it passes correct flags", func() {
			It("does not return an error", func() {
				Expect(update.Execute([]string{
					"--assets-file", someAssetsSpecFilePath,
				})).NotTo(HaveOccurred())
			})
		})

		When("it passes incorrect flags", func() {
			It("informs the user of bad flags", func() {
				Expect(update.Execute([]string{
					"--not-assets-file", someAssetsSpecFilePath,
				})).To(HaveOccurred())
			})
		})

		When("the stemcell version constraint bad", func() {
			var (
				updateErr error
				update    commands.Update
			)
			BeforeEach(func() {
				os.Remove(someAssetsSpecFilePath)
				contents := strings.ReplaceAll(initallAssetsYAMLFileContents, `"3586.*"`, "not-a-semver")
				ioutil.WriteFile(someAssetsSpecFilePath, []byte(contents), 0644)
				updateErr = update.Execute([]string{
					"--assets-file", someAssetsSpecFilePath,
				})
			})
			It("returns a descriptive error", func() {
				Expect(updateErr).To(HaveOccurred())
				Expect(updateErr.Error()).To(And(
					ContainSubstring("stemcell_constraint version error"),
					ContainSubstring("not-a-semver"),
				))
			})
		})

		When("given an assets.yml", func() {
			var (
				updateErr error
			)

			JustBeforeEach(func() {
				updateErr = update.Execute([]string{
					"--assets-file", someAssetsSpecFilePath,
				})
			})

			When("assets.yaml is missing", func() {
				BeforeEach(func() {
					Expect(os.Remove(someAssetsSpecFilePath)).NotTo(HaveOccurred())
				})
				It("returns a descriptive error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr).To(MatchError("could not read assets-file"))
				})
			})
			When("assets.yaml is missing", func() {
				BeforeEach(func() {
					os.Remove(someAssetsSpecFilePath)
					ioutil.WriteFile(someAssetsSpecFilePath, []byte(initallAssetsYAMLWithoutStemcellCriteraFileContents), 0644)
				})
				It("returns a descriptive error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr).To(MatchError(`stemcell OS ("") and/or version constraint ("") are not set`))
				})
			})
			When("the StemcellVersionsService returns an error", func() {
				BeforeEach(func() {
					stemcellsVersionsService.Returns.Err = errors.New("some-err")
				})
				It("returns a descriptive error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr.Error()).To(And(
						ContainSubstring("could not get stemcell versions"),
						ContainSubstring("some-err"),
					))
				})
			})
			When("the StemcellVersionsService returns a malformed version", func() {
				BeforeEach(func() {
					stemcellsVersionsService.Returns.Versions = append(stemcellsVersionsService.Returns.Versions, "bad-version")
				})
				It("ignores that version", func() {
					Expect(updateErr).ToNot(HaveOccurred())
				})
			})
			When("an updated version of a stemcell exists", func() {
				When("an assets.lock does not exists", func() {
					It("creates assets.lock file", func() {
						_, err := os.Stat(filepath.Join(tmpDir, "assets.lock"))
						Expect(err).NotTo(HaveOccurred())
					})
					It("writes updated assets.lock contents", func() {
						assetsLock, err := ioutil.ReadFile(someAssetsLockFilePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(assetsLock)).To(Equal(
							"########### DO NOT EDIT! ############\n" +
								"# This is a machine generated file, #\n" +
								"# update by running `kiln update`   #\n" +
								"#####################################\n" +
								"---\n" +
								"stemcell_criteria:\n" +
								"  os: ubuntu-trusty\n" +
								"  version: \"3586.7\"\n",
						))
					})
				})
				When("an assets.yaml has invalid yaml", func() {
					BeforeEach(func() {
						assetsYAML, err := os.OpenFile(someAssetsSpecFilePath, os.O_RDWR, 0644)
						Expect(err).NotTo(HaveOccurred())
						defer assetsYAML.Close()
						_, err = assetsYAML.WriteString("\n{{{[[[bla bla bla]]]}}}")
						Expect(err).NotTo(HaveOccurred())
					})
					It("returns a descriptive error", func() {
						Expect(updateErr).To(HaveOccurred())
						Expect(updateErr.Error()).To(ContainSubstring("could not parse yaml in assets-file"))
					})
				})
				When("an assets.lock already exists", func() {
					BeforeEach(func() {
						assetsLock, err := os.Create(someAssetsLockFilePath)
						Expect(err).NotTo(HaveOccurred())
						assetsLock.Write([]byte(initallAssetsLockFileContents))
						assetsLock.Close()
					})
					It("does not return an error", func() {
						Expect(updateErr).NotTo(HaveOccurred())
					})
					It("writes updated assets.lock contents", func() {
						assetsLock, err := ioutil.ReadFile(someAssetsLockFilePath)
						Expect(err).NotTo(HaveOccurred())
						Expect(string(assetsLock)).To(Equal(
							"########### DO NOT EDIT! ############\n" +
								"# This is a machine generated file, #\n" +
								"# update by running `kiln update`   #\n" +
								"#####################################\n" +
								"---\n" +
								"stemcell_criteria:\n" +
								"  os: ubuntu-trusty\n" +
								"  version: \"3586.7\"\n",
						))
					})
					When("an assets.lock has invalid yaml", func() {
						BeforeEach(func() {
							assetsLockFile, err := os.OpenFile(someAssetsLockFilePath, os.O_RDWR, 0644)
							Expect(err).NotTo(HaveOccurred())
							defer assetsLockFile.Close()
							_, err = assetsLockFile.WriteString("\n{{{[[[bla bla bla]]]}}}")
							Expect(err).NotTo(HaveOccurred())
						})
						It("returns a descriptive error", func() {
							Expect(updateErr).To(HaveOccurred())
							Expect(updateErr.Error()).To(ContainSubstring("did not find expected key"))
						})
					})
				})
			})
		})
	})
})

const (
	initallAssetsYAMLFileContents = `---
stemcell_criteria:
  os: ubuntu-trusty
  version: "3586.*"
`
	initallAssetsYAMLWithoutStemcellCriteraFileContents = `---
`
	initallAssetsLockFileContents = `---
stemcell_criteria:
  os: ubuntu-trusty
  version: "3586.1"
`
)

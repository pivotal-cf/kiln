package commands_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	. "github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
)

var _ = Describe("UpdateStemcell", func() {
	var _ jhanda.Command = UpdateStemcell{}
	When("Execute is called", func() {
		var (
			update                                        *UpdateStemcell
			tmpDir, someKilnfilePath, someKilfileLockPath string
			stemcellsVersionsService                      fakes.VersionsService
		)
		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "fetch-test")
			// fmt.Println(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			someKilnfilePath = filepath.Join(tmpDir, "Kilnfile")
			someKilfileLockPath = filepath.Join(tmpDir, "Kilnfile.lock")
			Expect(
				ioutil.WriteFile(someKilnfilePath, []byte(initallKilnfileYAMLFileContents), 0644),
			).NotTo(HaveOccurred())

			stemcellsVersionsService = fakes.VersionsService{}
			stemcellsVersionsService.VersionsCall.Returns.Versions = []string{
				"3580.0",
				"3586.7",
				"3586.2",
				"3586.3",
				"3588.0",
				"3587.1",
			}
			update = &UpdateStemcell{
				StemcellsVersionsService: &stemcellsVersionsService,
			}
		})
		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		When("it passes correct flags", func() {
			It("does not return an error", func() {
				Expect(update.Execute([]string{
					"--kilnfile", someKilnfilePath,
				})).NotTo(HaveOccurred())
			})
		})

		When("it passes incorrect flags", func() {
			It("informs the user of bad flags", func() {
				Expect(update.Execute([]string{
					"--not-kilnfile", someKilnfilePath,
				})).To(HaveOccurred())
			})
		})

		When("the stemcell version constraint is bad", func() {
			var (
				updateErr error
				update    UpdateStemcell
			)
			BeforeEach(func() {
				os.Remove(someKilnfilePath)
				contents := strings.ReplaceAll(initallKilnfileYAMLFileContents, `"3586.*"`, "not-a-semver")
				ioutil.WriteFile(someKilnfilePath, []byte(contents), 0644)
				update.StemcellsVersionsService = &fakes.VersionsService{}
				updateErr = update.Execute([]string{
					"--kilnfile", someKilnfilePath,
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

		When("given a Kilnfile", func() {
			var (
				updateErr error
			)

			JustBeforeEach(func() {
				updateErr = update.Execute([]string{
					"--kilnfile", someKilnfilePath,
				})
			})

			// happy paths vvv
			When("an updated version of a stemcell exists", func() {
				When("an Kilnfile.lock does not exists", func() {
					It("creates Kilnfile.lock file", func() {
						_, err := os.Stat(filepath.Join(tmpDir, "Kilnfile.lock"))
						Expect(err).NotTo(HaveOccurred())
					})
					It("writes updated Kilnfile.lock contents", func() {
						kilnfileLockFile, err := os.Open(someKilfileLockPath)
						Expect(err).NotTo(HaveOccurred())
						var kilnfileLock cargo.KilnfileLock
						err = yaml.NewDecoder(kilnfileLockFile).Decode(&kilnfileLock)
						Expect(err).NotTo(HaveOccurred())

						Expect(kilnfileLock).To(Equal(cargo.KilnfileLock{
							Releases: []cargo.Release{},
							Stemcell: cargo.Stemcell{
								OS:      "ubuntu-trusty",
								Version: "3586.7",
							},
						}))
					})
				})
				When("an Kilnfile.lock already exists", func() {
					BeforeEach(func() {
						kilnfileLock, createErr := os.Create(someKilfileLockPath)
						Expect(createErr).NotTo(HaveOccurred())
						kilnfileLock.Write([]byte(initallKilnfileLockFileContents))
						kilnfileLock.Close()
					})
					It("does not return an error", func() {
						Expect(updateErr).NotTo(HaveOccurred())
					})
					It("writes updated Kilnfile.lock contents", func() {
						kilnfileLockFile, err := os.Open(someKilfileLockPath)
						Expect(err).NotTo(HaveOccurred())
						var kilnfileLock cargo.KilnfileLock
						err = yaml.NewDecoder(kilnfileLockFile).Decode(&kilnfileLock)
						Expect(err).NotTo(HaveOccurred())

						Expect(kilnfileLock).To(Equal(cargo.KilnfileLock{
							Releases: []cargo.Release{},
							Stemcell: cargo.Stemcell{
								OS:      "ubuntu-trusty",
								Version: "3586.7",
							},
						}))
					})
					// happy paths ^^^

					When("a Kilnfile has invalid yaml", func() {
						BeforeEach(func() {
							kilnfile, err := os.OpenFile(someKilnfilePath, os.O_RDWR, 0644)
							Expect(err).NotTo(HaveOccurred())
							defer kilnfile.Close()
							_, err = kilnfile.WriteString("\n{{{[[[bla bla bla]]]}}}")
							Expect(err).NotTo(HaveOccurred())
						})
						It("returns a descriptive error", func() {
							Expect(updateErr).To(HaveOccurred())
							Expect(updateErr.Error()).To(ContainSubstring("could not parse yaml in kilnfile"))
						})
					})
					When("an Kilnfile.lock has invalid yaml", func() {
						BeforeEach(func() {
							kilnfileLockFile, err := os.OpenFile(someKilfileLockPath, os.O_RDWR, 0644)
							Expect(err).NotTo(HaveOccurred())
							defer kilnfileLockFile.Close()
							_, err = kilnfileLockFile.WriteString("\n{{{[[[bla bla bla]]]}}}")
							Expect(err).NotTo(HaveOccurred())
						})
						It("returns a descriptive error", func() {
							Expect(updateErr).To(HaveOccurred())
							Expect(updateErr.Error()).To(ContainSubstring("did not find expected key"))
						})
					})
				})
			})
			When("Kilnfile is missing", func() {
				BeforeEach(func() {
					Expect(os.Remove(someKilnfilePath)).NotTo(HaveOccurred())
				})
				It("returns a descriptive error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr).To(MatchError("could not read kilnfile"))
				})
			})
			When("Kilnfile is missing", func() {
				BeforeEach(func() {
					os.Remove(someKilnfilePath)
					ioutil.WriteFile(someKilnfilePath, []byte(initallKilnfileYAMLWithoutStemcellCriteraFileContents), 0644)
				})
				It("returns a descriptive error", func() {
					Expect(updateErr).To(HaveOccurred())
					Expect(updateErr).To(MatchError(`stemcell OS ("") and/or version constraint ("") are not set`))
				})
			})
			When("the StemcellVersionsService returns an error", func() {
				BeforeEach(func() {
					stemcellsVersionsService.VersionsCall.Returns.Err = errors.New("some-err")
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
					stemcellsVersionsService.VersionsCall.Returns.Versions = append(stemcellsVersionsService.VersionsCall.Returns.Versions, "bad-version")
				})
				It("ignores that version", func() {
					Expect(updateErr).ToNot(HaveOccurred())
				})
			})
		})
	})
})

const (
	initallKilnfileYAMLFileContents = `---
stemcell_criteria:
  os: ubuntu-trusty
  version: "3586.*"
`
	initallKilnfileYAMLWithoutStemcellCriteraFileContents = `---
`
	initallKilnfileLockFileContents = `---
stemcell_criteria:
  os: ubuntu-trusty
  version: "3586.1"
`
)

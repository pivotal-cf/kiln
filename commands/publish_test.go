package commands_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/go-pivnet/v2"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

var _ = Describe("Publish", func() {
	const (
		slug            = "elastic-runtime"
		publishDateBeta = "2019-10-28"
		publishDateRC   = "2019-11-11"
		publishDateGA   = "2019-12-06"

		defaultKilnFileBody = `---
slug: ` + slug + `
publish_dates:
  beta: ` + publishDateBeta + `
  rc: ` + publishDateRC + `
  ga: ` + publishDateGA
	)

	parseTime := func(date string) time.Time {
		t, _ := time.ParseInLocation(commands.PublishDateFormat, date, time.UTC)
		return t
	}

	var someVersion *semver.Version
	BeforeEach(func() {
		someVersion = semver.MustParse("2.8.0-build.111")
	})

	Describe("Execute", func() {
		When("on the happy-path", func() {
			var (
				publish          commands.Publish
				rs               *fakes.PivnetReleasesService
				pfs              *fakes.PivnetProductFilesService
				now              time.Time
				versionStr       string
				releasesOnPivnet []pivnet.Release
			)
			const releaseID = 123

			BeforeEach(func() {
				versionStr = "2.0.0-build.45"
				rs = &fakes.PivnetReleasesService{}
				pfs = &fakes.PivnetProductFilesService{}
				releasesOnPivnet = []pivnet.Release{}
			})

			JustBeforeEach(func() {
				if len(releasesOnPivnet) == 0 {
					releasesOnPivnet = []pivnet.Release{{Version: versionStr, ID: releaseID}}
				}
				rs.ListReturns(releasesOnPivnet, nil)

				rs.UpdateReturns(pivnet.Release{}, nil)

				fs := memfs.New()
				vf, _ := fs.Create("version")
				vf.Write([]byte(versionStr))
				vf.Close()

				kf, _ := fs.Create("Kilnfile")
				kf.Write([]byte(defaultKilnFileBody))
				kf.Close()

				publish = commands.Publish{
					FS:                        fs,
					PivnetReleaseService:      rs,
					PivnetProductFilesService: pfs,
					Now: func() time.Time {
						return now
					},
					OutLogger: log.New(ioutil.Discard, "", 0),
					ErrLogger: log.New(ioutil.Discard, "", 0),
				}
			})

			Context("before the beta window", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(rs.ListCallCount()).To(Equal(1))
					Expect(rs.ListArgsForCall(0)).To(Equal(slug))

					Expect(rs.UpdateCallCount()).To(Equal(1))
					{
						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-alpha.1"))
						Expect(r.ReleaseType).To(BeEquivalentTo("Alpha Release"))
						Expect(r.EndOfSupportDate).To(Equal(""))
						Expect(r.ReleaseDate).To(Equal("2019-10-27"))
					}
				})

				It("does not add a file to the release", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
				})
			})

			Context("during the beta window", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta)
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(rs.ListCallCount()).To(Equal(1))
					Expect(rs.ListArgsForCall(0)).To(Equal(slug))

					Expect(rs.UpdateCallCount()).To(Equal(1))
					{
						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-beta.1"))
						Expect(r.ReleaseType).To(BeEquivalentTo("Beta Release"))
						Expect(r.EndOfSupportDate).To(Equal(""))
						Expect(r.ReleaseDate).To(Equal(publishDateBeta))
					}
				})

				It("does not add a file to the release", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
				})
			})

			Context("during the rc window", func() {
				BeforeEach(func() {
					now = parseTime(publishDateRC)
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(rs.ListCallCount()).To(Equal(1))
					Expect(rs.ListArgsForCall(0)).To(Equal(slug))

					Expect(rs.UpdateCallCount()).To(Equal(1))
					{
						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-rc.1"))
						Expect(r.ReleaseType).To(BeEquivalentTo("Release Candidate"))
						Expect(r.EndOfSupportDate).To(Equal(""))
						Expect(r.ReleaseDate).To(Equal(publishDateRC))
					}
				})

				It("does not add a file to the release", func() {
					err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
				})
			})

			Context("during the ga window", func() {
				const (
					version20FileID = 42
					version21FileID = 43
				)

				BeforeEach(func() {
					pfs.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          40,
								Name:        "Uncle Bob's Magic Elixir",
								FileVersion: "2.0",
								FileType:    "Snake Oil",
							},
							{
								ID:          41,
								Name:        "Uncle Bob's Magic Elixir",
								FileVersion: "2.1",
								FileType:    "Snake Oil",
							},
							{
								ID:          version21FileID,
								Name:        "PCF Pivotal Application Service v2.1 OSL",
								FileVersion: "2.1",
								FileType:    "Open Source License",
							},
							{
								ID:          version20FileID,
								Name:        "PCF Pivotal Application Service v2.0 OSL",
								FileVersion: "2.0",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				Context("for a major release", func() {
					BeforeEach(func() {
						now = parseTime(publishDateGA)
						versionStr = "2.0.0-build.45"
					})

					It("updates Pivnet release with the determined version and release type", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.ListCallCount()).To(Equal(1))
						Expect(rs.ListArgsForCall(0)).To(Equal(slug))

						Expect(rs.UpdateCallCount()).To(Equal(1))
						{
							s, r := rs.UpdateArgsForCall(0)
							Expect(s).To(Equal(slug))
							Expect(r.Version).To(Equal("2.0.0"))
							Expect(r.ReleaseType).To(BeEquivalentTo("Major Release"))
							Expect(r.EndOfSupportDate).To(Equal("2020-09-30"))
							Expect(r.ReleaseDate).To(Equal(publishDateGA))
						}
					})

					It("adds the appropriate OSL file", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(1))
						productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(0)
						Expect(productSlug).To(Equal(slug))
						Expect(productReleaseID).To(Equal(releaseID))
						Expect(fileID).To(Equal(version20FileID))
					})
				})

				Context("for a minor release", func() {
					BeforeEach(func() {
						now = parseTime(publishDateGA)
						versionStr = "2.1.0-build.45"
					})

					It("updates Pivnet release with the determined version and release type", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.ListCallCount()).To(Equal(1))
						Expect(rs.ListArgsForCall(0)).To(Equal(slug))

						Expect(rs.UpdateCallCount()).To(Equal(1))
						{
							s, r := rs.UpdateArgsForCall(0)
							Expect(s).To(Equal(slug))
							Expect(r.Version).To(Equal("2.1.0"))
							Expect(r.ReleaseType).To(BeEquivalentTo("Minor Release"))
							Expect(r.EndOfSupportDate).To(Equal("2020-09-30"))
							Expect(r.ReleaseDate).To(Equal(publishDateGA))
						}
					})

					It("adds the appropriate OSL file", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(1))
						productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(0)
						Expect(productSlug).To(Equal(slug))
						Expect(productReleaseID).To(Equal(releaseID))
						Expect(fileID).To(Equal(version21FileID))
					})
				})

				Context("for a patch release", func() {
					var endOfSupportDate string
					BeforeEach(func() {
						now = parseTime(publishDateGA)
						versionStr = "2.1.1-build.45"
						endOfSupportDate = "2019-07-31"

						releasesOnPivnet = []pivnet.Release{
							{Version: versionStr, ID: releaseID},
							{Version: "2.1.0", EndOfSupportDate: endOfSupportDate},
							{Version: "2.1.1-build.1234"},
							{Version: "2.0.0", EndOfSupportDate: "2010-01-05"},
						}
					})

					It("updates Pivnet release with the determined version and release type", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.ListCallCount()).To(Equal(1))
						Expect(rs.ListArgsForCall(0)).To(Equal(slug))

						Expect(rs.UpdateCallCount()).To(Equal(1))
						{
							s, r := rs.UpdateArgsForCall(0)
							Expect(s).To(Equal(slug))
							Expect(r.Version).To(Equal("2.1.1"))
							Expect(r.ReleaseType).To(BeEquivalentTo("Maintenance Release"))
							Expect(r.EndOfSupportDate).To(Equal(endOfSupportDate))
							Expect(r.ReleaseDate).To(Equal(publishDateGA))
						}
					})

					It("adds the appropriate OSL file", func() {
						err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(1))
						productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(0)
						Expect(productSlug).To(Equal(slug))
						Expect(productReleaseID).To(Equal(releaseID))
						Expect(fileID).To(Equal(version21FileID))
					})
				})
			})

		})

		When("not the happy case", func() {
			var (
				publish commands.Publish
				now     time.Time
				fs      billy.Filesystem

				noVersionFile, noKilnFile     bool
				versionFileBody, kilnFileBody string
				releasesService               *fakes.PivnetReleasesService
				productFilesService           *fakes.PivnetProductFilesService

				executeArgs []string
			)

			BeforeEach(func() {
				publish = commands.Publish{}
				publish.Options.Kilnfile = "Kilnfile"
				publish.OutLogger = log.New(ioutil.Discard, "", 0)
				publish.ErrLogger = log.New(ioutil.Discard, "", 0)

				releasesService = &fakes.PivnetReleasesService{}
				productFilesService = &fakes.PivnetProductFilesService{}

				noVersionFile, noKilnFile = false, false
				fs = memfs.New()
				versionFileBody = someVersion.String()
				kilnFileBody = defaultKilnFileBody

				executeArgs = []string{"--pivnet-token", "SOME_TOKEN"}
			})

			JustBeforeEach(func() {
				if !noVersionFile {
					version, _ := fs.Create("version")
					version.Write([]byte(versionFileBody))
					version.Close()
				}

				if !noKilnFile {
					kilnFile, _ := fs.Create("Kilnfile")
					kilnFile.Write([]byte(kilnFileBody))
					kilnFile.Close()
				}

				publish.FS = fs
				publish.PivnetReleaseService = releasesService
				publish.PivnetProductFilesService = productFilesService
				publish.Now = func() time.Time {
					return now
				}
			})

			When("the release to be updated is not found", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
					releasesService.ListReturns([]pivnet.Release{{Version: "1.2.3-build.1"}}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(releasesService.ListCallCount()).To(Equal(1))
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("release with version " + someVersion.String() + " not found")))
				})
			})

			When("the version file contains an invalid semver", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
					versionFileBody = "not a banana"
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
				})
			})

			When("the Kilnfile does not exist", func() {
				BeforeEach(func() {
					noVersionFile = true
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("file does not exist")))
				})
			})

			When("the Kilnfile does not exist", func() {
				BeforeEach(func() {
					noKilnFile = true
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("file does not exist")))
				})
			})

			When("the Kilnfile contains invalid YAML", func() {
				BeforeEach(func() {
					kilnFileBody = "---> bad yaml file <---"
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
				})
			})

			When("there is bad yaml in the file", func() {
				BeforeEach(func() {
					kilnFileBody = `}`
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("yaml:")))
				})
			})

			When("a date is not properly formatted", func() {
				BeforeEach(func() {
					kilnFileBody = `---
publish_dates:
  beta: "ERROR"
`
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("parsing time")))
				})
			})

			When("there is an error fetching product files from Pivnet", func() {
				BeforeEach(func() {
					now = parseTime(publishDateGA)
					releasesService.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					productFilesService.ListReturns(nil, errors.New("bad stuff happened"))
				})

				It("returns an error and makes no changes", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())

					Expect(releasesService.UpdateCallCount()).To(Equal(0))
					Expect(productFilesService.ListCallCount()).To(Equal(1))
					Expect(productFilesService.AddToReleaseCallCount()).To(Equal(0))
					Expect(err).To(MatchError("bad stuff happened"))
				})
			})

			When("there the necessary license file doesn't exist on Pivnet", func() {
				BeforeEach(func() {
					now = parseTime(publishDateGA)
					releasesService.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					productFilesService.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          42,
								Name:        "PCF Pivotal Application Service v2.1 OSL",
								FileVersion: "2.1",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				It("returns an error and makes no changes", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("file doesn't exist")))

					Expect(releasesService.UpdateCallCount()).To(Equal(0))
					Expect(productFilesService.ListCallCount()).To(Equal(1))
					Expect(productFilesService.AddToReleaseCallCount()).To(Equal(0))
				})
			})

			When("there is an error adding the license file to the release on Pivnet", func() {
				BeforeEach(func() {
					now = parseTime(publishDateGA)
					releasesService.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					productFilesService.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          42,
								Name:        "PCF Pivotal Application Service v2.8 OSL",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
					productFilesService.AddToReleaseReturns(errors.New("more bad stuff happened"))
				})

				It("returns an error and makes no changes", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())

					Expect(releasesService.UpdateCallCount()).To(Equal(0))
					Expect(productFilesService.ListCallCount()).To(Equal(1))
					Expect(productFilesService.AddToReleaseCallCount()).To(Equal(1))
					Expect(err).To(MatchError("more bad stuff happened"))
				})
			})

			When("a release on PivNet has an invalid version", func() {
				BeforeEach(func() {
					now = parseTime(publishDateGA)
					releasesService.ListReturns([]pivnet.Release{
						{Version: someVersion.String()},
						{Version: "invalid version"},
					}, nil)
					productFilesService.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          42,
								Name:        "PCF Pivotal Application Service v2.8 OSL",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("Invalid Semantic Version")))
				})
			})
		})
	})

	Describe("DetermineVersion", func() {
		var (
			publish commands.Publish

			releases        []pivnet.Release
			releasesService *fakes.PivnetReleasesService

			now time.Time
		)

		BeforeEach(func() {
			releasesService = &fakes.PivnetReleasesService{}
			publish = commands.Publish{}
			publish.Options.Kilnfile = "Kilnfile"
		})

		JustBeforeEach(func() {
			publish.PivnetReleaseService = releasesService
			publish.Now = func() time.Time {
				return now
			}
		})

		When("patch version is more than 0", func() {
			It("returns the version number without the prerelease segments", func() {
				got, err := publish.DetermineVersion(releases, "", semver.MustParse("1.2.3-build.45"))
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal(semver.MustParse("1.2.3")))
				Expect(releasesService.ListCallCount()).To(Equal(0))
				Expect(releasesService.UpdateCallCount()).To(Equal(0))
			})
		})

		When("patch version is 0", func() {
			BeforeEach(func() {
				var err error
				// TODO: Consider removing this beforeeach
				resBody, err := os.Open("testdata/elastic-runtime-releases.json")
				Expect(err).NotTo(HaveOccurred())
				var file struct {
					Releases []pivnet.Release `json:"releases"`
				}
				err = json.NewDecoder(resBody).Decode(&file)
				releases = file.Releases
				Expect(err).NotTo(HaveOccurred())
			})

			When("there has not been an alpha release yet", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("returns alpha 1 as the version number", func() {
					got, err := publish.DetermineVersion(releases, "alpha", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-alpha.1")))
				})
			})

			When("there has been two alpha releases and we are in the alpha window", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1"}, {Version: "2.8.0-alpha.2"}}
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("returns alpha 3 as the version number", func() {
					got, err := publish.DetermineVersion(releases, "alpha", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-alpha.3")))
				})
			})

			When("a beta prerelease exists and we are in the beta window", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-beta.1"}, {Version: "2.8.0-alpha.1"}}
					now = parseTime(publishDateBeta)
				})

				It("returns beta 2 as the prerelease version number", func() {
					got, err := publish.DetermineVersion(releases, "beta", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-beta.2")))
				})
			})

			When("the response from pivnet is an unsorted version list and we are in the rc window", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1"}, {Version: "2.8.0-rc.1"}, {Version: "2.8.0-beta.1"}, {Version: "2.8.0-beta.2"}}
					now = parseTime(publishDateRC)
					now.Add(time.Hour * 24 * 2) // put the date into middle of the window
				})

				It("returns the next beta number as the prerelease version number", func() {
					got, err := publish.DetermineVersion(releases, "rc", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-rc.2")))
				})
			})

			When("the release window is in beta", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-rc.1"}, {Version: "2.8.0-alpha.1"}, {Version: "2.8.0-beta.1"}, {Version: "2.8.0-beta.2"}}
					now = parseTime(publishDateBeta)
				})

				It("returns the next beta number as the prerelease version number", func() {
					got, err := publish.DetermineVersion(releases, "beta", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-beta.3")))
				})
			})

			When("the release window is in rc", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1"}, {Version: "2.8.0-beta.1"}, {Version: "2.8.0-beta.2"}}
					now = parseTime(publishDateRC)
				})

				It("returns the first rc number", func() {
					got, err := publish.DetermineVersion(releases, "rc", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0-rc.1")))
				})
			})

			When("the release window is in GA", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1"}, {Version: "2.8.0-beta.1"}, {Version: "2.8.0-beta.2"}, {Version: "2.8.0-rc.1"}}
					now = parseTime(publishDateGA)
				})

				It("returns a version without a prelease part", func() {
					got, err := publish.DetermineVersion(releases, "ga", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal(semver.MustParse("2.8.0")))
				})
			})

			When("the prerelease number is malformed", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1a"}, {Version: "2.8.0-alpha.1b"}}
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("returns an error", func() {
					_, err := publish.DetermineVersion(releases, "", someVersion)
					Expect(err).To(HaveOccurred())
				})
			})

			When("the prerelease does not have a dot", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha1"}}
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("returns an error", func() {
					_, err := publish.DetermineVersion(releases, "", someVersion)
					Expect(err).To(HaveOccurred())
				})
			})

			When("the Now func is not set", func() {
				It("it does not panic", func() {
					publish.Now = nil
					publish.DetermineVersion(releases, "", someVersion)
				})
			})
		})
	})

	Describe("Kilnfile", func() {
		Describe("ReleaseWindow", func() {
			var (
				fs billy.Filesystem

				kilnFileBody string
				noKilnFile   bool
				now          time.Time
				kilnfile     commands.Kilnfile
			)

			BeforeEach(func() {
				noKilnFile = false
				fs = memfs.New()
				now = time.Now()
				kilnFileBody = defaultKilnFileBody
			})

			JustBeforeEach(func() {
				if !noKilnFile {
					kilnFile, _ := fs.Create("Kilnfile")
					kilnFile.Write([]byte(kilnFileBody))
					kilnFile.Close()
				}

				kilnfile = commands.Kilnfile{}
				kilnfile.PublishDates.RC = commands.Date{parseTime(publishDateRC)}
				kilnfile.PublishDates.Beta = commands.Date{parseTime(publishDateBeta)}
				kilnfile.PublishDates.GA = commands.Date{parseTime(publishDateGA)}
			})

			When("the window is before beta", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta).Add(-24 * time.Hour)
				})

				It("returns alpha", func() {
					window, err := kilnfile.ReleaseWindow(now)
					Expect(err).NotTo(HaveOccurred())
					Expect(window).To(Equal("alpha"))
				})
			})

			When("the window is in beta", func() {
				BeforeEach(func() {
					now = parseTime(publishDateBeta)
				})

				It("returns beta", func() {
					window, err := kilnfile.ReleaseWindow(now)
					Expect(err).NotTo(HaveOccurred())
					Expect(window).To(Equal("beta"))
				})
			})

			When("the window is in rc", func() {
				BeforeEach(func() {
					now = parseTime(publishDateRC)
				})

				It("returns rc", func() {
					window, err := kilnfile.ReleaseWindow(now)
					Expect(err).NotTo(HaveOccurred())
					Expect(window).To(Equal("rc"))
				})
			})

			When("the window is GA", func() {
				BeforeEach(func() {
					now = parseTime(publishDateGA)
				})

				It("returns ga", func() {
					window, err := kilnfile.ReleaseWindow(now)
					Expect(err).NotTo(HaveOccurred())
					Expect(window).To(Equal("ga"))
				})
			})
		})
	})
})

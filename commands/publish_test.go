package commands_test

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pivnet "github.com/pivotal-cf/go-pivnet"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

var _ = Describe("Publish", func() {
	const (
		slug             = "elastic-runtime"
		publishDateAlpha = "2019-10-04"
		publishDateBeta  = "2019-10-28"
		publishDateRC    = "2019-11-11"
		publishDateGA    = "2019-12-06"

		defaultKilnFileBody = `---
slug: ` + slug + `
publish_dates:
  alpha: ` + publishDateAlpha + `
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
		It("updates Pivnet release with the determined version", func() {
			release := pivnet.Release{Version: "2.0.0-build.45", ID: 123}
			rs := &fakes.PivnetReleasesService{}
			rs.ListReturns([]pivnet.Release{release}, nil)
			rs.UpdateReturns(pivnet.Release{}, nil)

			fs := memfs.New()
			vf, _ := fs.Create("version")
			vf.Write([]byte("2.0.0-build.45"))
			vf.Close()

			kf, _ := fs.Create("Kilnfile")
			kf.Write([]byte(defaultKilnFileBody))
			kf.Close()

			publish := commands.Publish{
				FS:     fs,
				Pivnet: rs,
				Now: func() time.Time {
					return parseTime(publishDateAlpha)
				},
				OutLogger: log.New(ioutil.Discard, "", 0),
				ErrLogger: log.New(ioutil.Discard, "", 0),
			}
			err := publish.Execute([]string{"--pivnet-token", "SOME_TOKEN"})
			Expect(err).NotTo(HaveOccurred())

			Expect(rs.ListCallCount()).To(Equal(1))
			Expect(rs.ListArgsForCall(0)).To(Equal(slug))

			Expect(rs.UpdateCallCount()).To(Equal(1))
			{
				s, r := rs.UpdateArgsForCall(0)
				Expect(s).To(Equal(slug))
				Expect(r.Version).To(Equal("2.0.0-alpha.1"))
			}
		})

		When("not the happy case", func() {
			var (
				publish commands.Publish
				now     time.Time
				fs      billy.Filesystem

				noVersionFile, noKilnFile     bool
				versionFileBody, kilnFileBody string
				releasesService               *fakes.PivnetReleasesService

				executeArgs []string
			)

			BeforeEach(func() {
				publish = commands.Publish{}
				publish.Options.Kilnfile = "Kilnfile"
				publish.OutLogger = log.New(ioutil.Discard, "", 0)
				publish.ErrLogger = log.New(ioutil.Discard, "", 0)

				releasesService = &fakes.PivnetReleasesService{}

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
				publish.Pivnet = releasesService
				publish.Now = func() time.Time {
					return now
				}
			})

			When("the release to be updated is not found", func() {
				BeforeEach(func() {
					now = parseTime(publishDateAlpha)
					releasesService.ListReturns([]pivnet.Release{{Version: "1.2.3-build.1"}}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(releasesService.ListCallCount()).To(Equal(1))
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("release with version " + someVersion.String() + " not found on")))
				})
			})

			When("the version file contains an invalid semver", func() {
				BeforeEach(func() {
					now = parseTime(publishDateAlpha)
					versionFileBody = "not a banana"
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
				})
			})

			When("the kiln file does not exist", func() {
				BeforeEach(func() {
					noVersionFile = true
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("file does not exist")))
				})
			})

			When("the kiln file does not exist", func() {
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
  alpha: "ERROR"
`
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("parsing time")))
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
			publish.Pivnet = releasesService
			publish.Now = func() time.Time {
				return now
			}
		})

		When("patch version is more than 0", func() {
			It("returns the version number without the prerelease segments", func() {
				got, err := publish.DetermineVersion(releases, "", semver.MustParse("1.2.3-build.45"))
				Expect(err).NotTo(HaveOccurred())
				Expect(got).To(Equal("1.2.3"))
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
					now = parseTime(publishDateAlpha)
				})

				It("returns alpha 1 as the version number", func() {
					got, err := publish.DetermineVersion(releases, "alpha", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal("2.8.0-alpha.1"))
				})
			})

			When("there has been two alpha releases and we are in the alpha window", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1"}, {Version: "2.8.0-alpha.2"}}
					now = parseTime(publishDateAlpha)
				})

				It("returns alpha 3 as the version number", func() {
					got, err := publish.DetermineVersion(releases, "alpha", someVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(got).To(Equal("2.8.0-alpha.3"))
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
					Expect(got).To(Equal("2.8.0-beta.2"))
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
					Expect(got).To(Equal("2.8.0-rc.2"))
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
					Expect(got).To(Equal("2.8.0-beta.3"))
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
					Expect(got).To(Equal("2.8.0-rc.1"))
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
					Expect(got).To(Equal("2.8.0"))
				})
			})

			When("the prerelease number is malformed", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha.1a"}, {Version: "2.8.0-alpha.1b"}}
					now = parseTime(publishDateAlpha)
				})

				It("returns an error", func() {
					_, err := publish.DetermineVersion(releases, "", someVersion)
					Expect(err).To(HaveOccurred())
				})
			})

			When("the prerelease does not have a dot", func() {
				BeforeEach(func() {
					releases = []pivnet.Release{{Version: "2.8.0-alpha1"}}
					now = parseTime(publishDateAlpha)
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
				kilnfile.PublishDates.Alpha = commands.Date{parseTime(publishDateAlpha)}
				kilnfile.PublishDates.RC = commands.Date{parseTime(publishDateRC)}
				kilnfile.PublishDates.Beta = commands.Date{parseTime(publishDateBeta)}
				kilnfile.PublishDates.GA = commands.Date{parseTime(publishDateGA)}
			})

			When("the window is before alpha", func() {
				BeforeEach(func() {
					now = parseTime(publishDateAlpha)
					now = now.Add(-time.Hour * 24 * 5)
				})

				It("returns internal", func() {
					window, err := kilnfile.ReleaseWindow(now)
					Expect(err).NotTo(HaveOccurred())
					Expect(window).To(Equal("internal"))
				})
			})

			When("the window is in alpha", func() {
				BeforeEach(func() {
					now = parseTime(publishDateAlpha)
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

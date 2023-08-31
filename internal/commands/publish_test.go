package commands_test

import (
	"errors"
	"io"
	"log"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/pivotal-cf/go-pivnet/v7"

	"github.com/pivotal-cf/kiln/internal/commands"
	commandsFakes "github.com/pivotal-cf/kiln/internal/commands/fakes"
)

var _ = Describe("Publish", func() {
	const (
		slug           = "elastic-runtime"
		userGroup1Name = "Dell/EMC Early Access Group"
		userGroup1ID   = 123
		userGroup2Name = "PCF R&D"
		userGroup2ID   = 456

		defaultKilnFileBody = `---
slug: ` + slug + `
pre_ga_user_groups:
  - ` + userGroup1Name + `
  - ` + userGroup2Name + `
`
	)

	var someVersion *semver.Version
	BeforeEach(func() {
		someVersion = semver.MustParse("2.8.0-build.111")
	})

	Describe("Execute", func() {
		When("on the happy-path", func() {
			var (
				publish                    commands.Publish
				rs                         *commandsFakes.PivnetReleasesService
				pfs                        *commandsFakes.PivnetProductFilesService
				ugs                        *commandsFakes.PivnetUserGroupsService
				releaseUpgradePathsService *commandsFakes.PivnetReleaseUpgradePathsService
				releaseDependenciesService *commandsFakes.PivnetReleaseDependenciesService
				now                        time.Time
				versionStr                 string
				releasesOnPivnet           []pivnet.Release
				outLoggerBuffer            strings.Builder
			)
			const releaseID = 123

			BeforeEach(func() {
				versionStr = "2.0.0-build.45"
				rs = new(commandsFakes.PivnetReleasesService)
				pfs = new(commandsFakes.PivnetProductFilesService)
				ugs = new(commandsFakes.PivnetUserGroupsService)
				releaseUpgradePathsService = new(commandsFakes.PivnetReleaseUpgradePathsService)
				releaseUpgradePathsService.GetReturns([]pivnet.ReleaseUpgradePath{{}}, nil)
				releaseDependenciesService = new(commandsFakes.PivnetReleaseDependenciesService)
				releaseDependenciesService.ListReturns([]pivnet.ReleaseDependency{{}}, nil)
				releasesOnPivnet = []pivnet.Release{}
				now = time.Now()
				outLoggerBuffer = strings.Builder{}
			})

			JustBeforeEach(func() {
				if len(releasesOnPivnet) == 0 {
					releasesOnPivnet = []pivnet.Release{{Version: versionStr, ID: releaseID}}
				}
				rs.ListReturns(releasesOnPivnet, nil)

				rs.UpdateReturns(pivnet.Release{ID: releaseID}, nil)

				ugs.ListReturns([]pivnet.UserGroup{
					{ID: userGroup1ID, Name: userGroup1Name},
					{ID: 123, Name: "Ignore me!"},
					{ID: userGroup2ID, Name: userGroup2Name},
				}, nil)

				fs := memfs.New()
				vf, _ := fs.Create("version")
				_, _ = vf.Write([]byte(versionStr))
				defer closeAndIgnoreError(vf)

				kf, _ := fs.Create("Kilnfile")
				_, _ = kf.Write([]byte(defaultKilnFileBody))
				defer closeAndIgnoreError(kf)

				publish = commands.Publish{
					FS:                               fs,
					PivnetReleaseService:             rs,
					PivnetProductFilesService:        pfs,
					PivnetUserGroupsService:          ugs,
					PivnetReleaseUpgradePathsService: releaseUpgradePathsService,
					PivnetReleaseDependenciesService: releaseDependenciesService,
					Now: func() time.Time {
						return now
					},
					OutLogger: log.New(&outLoggerBuffer, "", 0),
					ErrLogger: log.New(io.Discard, "", 0),
				}
			})

			Context("during the alpha window", func() {
				var args []string

				BeforeEach(func() {
					args = []string{"--window", "alpha", "--pivnet-token", "SOME_TOKEN"}
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute(args)
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
						Expect(r.ReleaseDate).To(Equal(now.Format("2006-01-02")))
						Expect(r.Availability).To(Equal("Selected User Groups Only"))
					}
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.0.0-alpha.1"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Alpha Release"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: Selected User Groups Only"))
				})

				It("does not add a file to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("  License file: None, pre-GA release"))
				})

				It("adds the pre-GA user groups to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(ugs.ListCallCount()).To(Equal(1))

					Expect(ugs.AddToReleaseCallCount()).To(Equal(2))

					s, rid, ugid := ugs.AddToReleaseArgsForCall(0)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup1ID))

					s, rid, ugid = ugs.AddToReleaseArgsForCall(1)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup2ID))

					Expect(outLoggerBuffer.String()).To(ContainSubstring("Granting access to groups..."))
					Expect(outLoggerBuffer.String()).To(ContainSubstring(userGroup1Name))
					Expect(outLoggerBuffer.String()).To(ContainSubstring(userGroup2Name))

					Expect(outLoggerBuffer.String()).To(ContainSubstring("Successfully published tile."))
				})

				Context("when previous alphas have been published", func() {
					BeforeEach(func() {
						releasesOnPivnet = []pivnet.Release{
							{Version: versionStr, ID: releaseID},
							{Version: "2.0.0-alpha.456"},
						}
					})

					It("publishes with a version that increments the alpha number", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-alpha.457"))
					})
				})

				Context("when the --security-fix flag is given", func() {
					BeforeEach(func() {
						args = append(args, "--security-fix")
					})

					It("sets the correct release type", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.UpdateCallCount()).To(Equal(1))
						_, r := rs.UpdateArgsForCall(0)
						Expect(r.ReleaseType).To(BeEquivalentTo("Alpha Release"))
					})
				})
			})

			Context("during the beta window", func() {
				var args []string

				BeforeEach(func() {
					args = []string{"--window", "beta", "--pivnet-token", "SOME_TOKEN"}
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute(args)
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
						Expect(r.ReleaseDate).To(Equal(now.Format("2006-01-02")))
						Expect(r.Availability).To(Equal("Selected User Groups Only"))
					}
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.0.0-beta.1"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Beta Release"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: Selected User Groups Only"))
				})

				It("does not add a file to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("  License file: None, pre-GA release"))
				})

				It("adds the pre-GA user groups to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(ugs.ListCallCount()).To(Equal(1))

					Expect(ugs.AddToReleaseCallCount()).To(Equal(2))

					s, rid, ugid := ugs.AddToReleaseArgsForCall(0)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup1ID))

					s, rid, ugid = ugs.AddToReleaseArgsForCall(1)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup2ID))
				})

				Context("when previous betas have been published", func() {
					BeforeEach(func() {
						releasesOnPivnet = []pivnet.Release{
							{Version: versionStr, ID: releaseID},
							{Version: "2.0.0-beta.123"},
						}
					})

					It("publishes with a version that increments the alpha number", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-beta.124"))
					})
				})

				Context("when the --security-fix flag is given", func() {
					BeforeEach(func() {
						args = append(args, "--security-fix")
					})

					It("sets the correct release type", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.UpdateCallCount()).To(Equal(1))
						_, r := rs.UpdateArgsForCall(0)
						Expect(r.ReleaseType).To(BeEquivalentTo("Beta Release"))
					})
				})
			})

			Context("during the rc window", func() {
				var args []string

				BeforeEach(func() {
					args = []string{"--window", "rc", "--pivnet-token", "SOME_TOKEN"}
				})

				It("updates Pivnet release with the determined version and release type", func() {
					err := publish.Execute(args)
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
						Expect(r.ReleaseDate).To(Equal(now.Format("2006-01-02")))
						Expect(r.Availability).To(Equal("Selected User Groups Only"))
					}
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.0.0-rc.1"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Release Candidate"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: Selected User Groups Only"))
				})

				It("does not add a file to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("  License file: None, pre-GA release"))
				})

				It("adds the pre-GA user groups to the release", func() {
					err := publish.Execute(args)
					Expect(err).NotTo(HaveOccurred())

					Expect(ugs.ListCallCount()).To(Equal(1))

					Expect(ugs.AddToReleaseCallCount()).To(Equal(2))

					s, rid, ugid := ugs.AddToReleaseArgsForCall(0)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup1ID))

					s, rid, ugid = ugs.AddToReleaseArgsForCall(1)
					Expect(s).To(Equal(s))
					Expect(rid).To(Equal(releaseID))
					Expect(ugid).To(Equal(userGroup2ID))

					Expect(outLoggerBuffer.String()).To(ContainSubstring("Granting access to groups..."))
					Expect(outLoggerBuffer.String()).To(ContainSubstring(userGroup1Name))
					Expect(outLoggerBuffer.String()).To(ContainSubstring(userGroup2Name))
				})

				Context("when previous release candidates have been published", func() {
					BeforeEach(func() {
						releasesOnPivnet = []pivnet.Release{
							{Version: versionStr, ID: releaseID},
							{Version: "2.0.0-rc.2"},
							{Version: "2.0.0-rc.1"},
						}
					})

					It("publishes with a version that increments the alpha number", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.0.0-rc.3"))
					})
				})

				Context("when the --security-fix flag is given", func() {
					BeforeEach(func() {
						args = append(args, "--security-fix")
					})

					It("sets the correct release type", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.UpdateCallCount()).To(Equal(1))
						_, r := rs.UpdateArgsForCall(0)
						Expect(r.ReleaseType).To(BeEquivalentTo("Release Candidate"))
					})
				})
			})

			Context("during the ga window", func() {
				const (
					version20OslFileID        = 42
					version21OslFileID        = 43
					version21OsmPartOneFileId = 44
					version21OsmPartTwoFileId = 45
					version20OsmFileId        = 46
				)
				var (
					args             []string
					endOfSupportDate string
				)

				BeforeEach(func() {
					args = []string{"--window", "ga", "--pivnet-token", "SOME_TOKEN"}

					now = time.Date(2016, 5, 4, 3, 2, 1, 0, time.Local)
					endOfSupportDate = "2017-02-28" // by default, PivNet does not have EOGS: now + 300 days

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
								ID:          version21OslFileID,
								Name:        "PCF Pivotal Application Service v2.1 OSL",
								FileVersion: "2.1",
								FileType:    "Open Source License",
							},
							{
								ID:          version20OslFileID,
								Name:        "PCF Pivotal Application Service v2.0 OSL",
								FileVersion: "2.0",
								FileType:    "Open Source License",
							},
							{
								ID:          version21OsmPartOneFileId,
								Name:        "PCF Pivotal Application Service v2.1 OSM Part 1/2",
								FileVersion: "2.1",
								FileType:    "Open Source License",
							},
							{
								ID:          version21OsmPartTwoFileId,
								Name:        "PCF Pivotal Application Service v2.1 OSM Part 2/2",
								FileVersion: "2.1",
								FileType:    "Open Source License",
							},
							{
								ID:          version20OsmFileId,
								Name:        "PCF Pivotal Application Service v2.0 OSM",
								FileVersion: "2.0",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				Context("for a major release", func() {
					BeforeEach(func() {
						versionStr = "2.0.0-build.45"
					})

					It("updates Pivnet release with the determined version and release type", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.ListCallCount()).To(Equal(1))
						Expect(rs.ListArgsForCall(0)).To(Equal(slug))

						Expect(rs.UpdateCallCount()).To(Equal(1))
						{
							s, r := rs.UpdateArgsForCall(0)
							Expect(s).To(Equal(slug))
							Expect(r.Version).To(Equal("2.0.0"))
							Expect(r.ReleaseType).To(BeEquivalentTo("Major Release"))
							Expect(r.EndOfSupportDate).To(Equal(endOfSupportDate))
							Expect(r.ReleaseDate).To(Equal("2016-05-04"))
							Expect(r.Availability).To(Equal("All Users"))
						}
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.0.0"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Major Release"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("EOGS date: %s", endOfSupportDate))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: All Users"))
					})

					It("attaches all license files to the release that match its major and minor", func() {
						expectedProductFileIds := []int{version20OslFileID, version20OsmFileId}

						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(2))

						var productFileIds []int
						for i := 0; i <= 1; i++ {
							productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(i)
							Expect(productSlug).To(Equal(slug))
							Expect(productReleaseID).To(Equal(releaseID))
							productFileIds = append(productFileIds, fileID)
						}
						Expect(productFileIds).To(Equal(expectedProductFileIds))

						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.0 OSL"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.0 OSM"))
					})

					Context("when the --security-fix flag is given", func() {
						BeforeEach(func() {
							args = append(args, "--security-fix")
						})

						It("sets the correct release type", func() {
							err := publish.Execute(args)
							Expect(err).NotTo(HaveOccurred())

							Expect(rs.UpdateCallCount()).To(Equal(1))
							_, r := rs.UpdateArgsForCall(0)
							Expect(r.ReleaseType).To(BeEquivalentTo("Major Release"))
						})
					})
				})

				Context("for a minor release", func() {
					BeforeEach(func() {
						versionStr = "2.1.0-build.45"
					})

					It("updates Pivnet release with the determined version and release type", func() {
						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(rs.ListCallCount()).To(Equal(1))
						Expect(rs.ListArgsForCall(0)).To(Equal(slug))

						Expect(rs.UpdateCallCount()).To(Equal(1))
						{
							s, r := rs.UpdateArgsForCall(0)
							Expect(s).To(Equal(slug))
							Expect(r.Version).To(Equal("2.1.0"))
							Expect(r.ReleaseType).To(BeEquivalentTo("Minor Release"))
							Expect(r.EndOfSupportDate).To(Equal(endOfSupportDate))
							Expect(r.ReleaseDate).To(Equal("2016-05-04"))
							Expect(r.Availability).To(Equal("All Users"))
						}
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.1.0"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Minor Release"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("EOGS date: %s", endOfSupportDate))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: All Users"))
					})

					It("attaches all license files to the release that match its major and minor", func() {
						expectedProductFileIds := []int{version21OslFileID, version21OsmPartOneFileId, version21OsmPartTwoFileId}

						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(3))

						var productFileIds []int
						for i := 0; i <= 2; i++ {
							productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(i)
							Expect(productSlug).To(Equal(slug))
							Expect(productReleaseID).To(Equal(releaseID))
							productFileIds = append(productFileIds, fileID)
						}
						Expect(productFileIds).To(Equal(expectedProductFileIds))

						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSL"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSM Part 1/2"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSM Part 2/2"))
					})

					Context("when the --security-fix flag is given", func() {
						BeforeEach(func() {
							args = append(args, "--security-fix")
						})

						It("sets the correct release type", func() {
							err := publish.Execute(args)
							Expect(err).NotTo(HaveOccurred())

							Expect(rs.UpdateCallCount()).To(Equal(1))
							_, r := rs.UpdateArgsForCall(0)
							Expect(r.ReleaseType).To(BeEquivalentTo("Minor Release"))
						})
					})
				})

				Context("for a patch release", func() {
					BeforeEach(func() {
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
						err := publish.Execute(args)
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
							Expect(r.ReleaseDate).To(Equal("2016-05-04"))
							Expect(r.Availability).To(Equal("All Users"))
						}
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.1.1"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Maintenance Release"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Release date: %s", now.Format("2006-01-02")))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("EOGS date: %s", endOfSupportDate))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("Availability: All Users"))
					})

					It("attaches all license files to the release that match its major and minor", func() {
						expectedProductFileIds := []int{version21OslFileID, version21OsmPartOneFileId, version21OsmPartTwoFileId}

						err := publish.Execute(args)
						Expect(err).NotTo(HaveOccurred())

						Expect(pfs.AddToReleaseCallCount()).To(Equal(3))

						var productFileIds []int
						for i := 0; i <= 2; i++ {
							productSlug, productReleaseID, fileID := pfs.AddToReleaseArgsForCall(i)
							Expect(productSlug).To(Equal(slug))
							Expect(productReleaseID).To(Equal(releaseID))
							productFileIds = append(productFileIds, fileID)
						}
						Expect(productFileIds).To(Equal(expectedProductFileIds))

						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSL"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSM Part 1/2"))
						Expect(outLoggerBuffer.String()).To(ContainSubstring("License file: PCF Pivotal Application Service v2.1 OSM Part 2/2"))
					})

					Context("when the --security-fix flag is given", func() {
						BeforeEach(func() {
							args = append(args, "--security-fix")
						})

						It("sets the correct release type", func() {
							err := publish.Execute(args)
							Expect(err).NotTo(HaveOccurred())

							Expect(rs.UpdateCallCount()).To(Equal(1))
							_, r := rs.UpdateArgsForCall(0)
							Expect(r.ReleaseType).To(BeEquivalentTo("Security Release"))
						})
					})
				})
			})
		})

		When("the sad/unhappy case", func() {
			var (
				publish commands.Publish
				now     time.Time
				fs      billy.Filesystem

				noVersionFile, noKilnFile     bool
				versionFileBody, kilnFileBody string
				rs                            *commandsFakes.PivnetReleasesService
				pfs                           *commandsFakes.PivnetProductFilesService
				ugs                           *commandsFakes.PivnetUserGroupsService
				releaseUpgradePathsService    *commandsFakes.PivnetReleaseUpgradePathsService
				releaseDependenciesService    *commandsFakes.PivnetReleaseDependenciesService

				executeArgs     []string
				outLoggerBuffer strings.Builder
				errLoggerBuffer strings.Builder
			)

			BeforeEach(func() {
				publish = commands.Publish{}
				publish.Options.Kilnfile = "Kilnfile"
				outLoggerBuffer = strings.Builder{}
				publish.OutLogger = log.New(&outLoggerBuffer, "", 0)
				publish.ErrLogger = log.New(&errLoggerBuffer, "", 0)

				rs = new(commandsFakes.PivnetReleasesService)
				pfs = new(commandsFakes.PivnetProductFilesService)
				ugs = new(commandsFakes.PivnetUserGroupsService)
				releaseUpgradePathsService = new(commandsFakes.PivnetReleaseUpgradePathsService)
				releaseUpgradePathsService.GetReturns([]pivnet.ReleaseUpgradePath{{}}, nil)

				releaseDependenciesService = new(commandsFakes.PivnetReleaseDependenciesService)
				releaseDependenciesService.ListReturns([]pivnet.ReleaseDependency{{}}, nil)

				noVersionFile, noKilnFile = false, false
				fs = memfs.New()
				kilnFileBody = defaultKilnFileBody

				executeArgs = []string{"--pivnet-token", "SOME_TOKEN", "--window", "ga"}
			})

			JustBeforeEach(func() {
				versionFileBody = someVersion.String()

				if !noVersionFile {
					version, _ := fs.Create("version")
					_, _ = version.Write([]byte(versionFileBody))
					defer closeAndIgnoreError(version)
				}

				if !noKilnFile {
					kilnFile, _ := fs.Create("Kilnfile")
					_, _ = kilnFile.Write([]byte(kilnFileBody))
					defer closeAndIgnoreError(kilnFile)
				}

				publish.FS = fs
				publish.PivnetReleaseService = rs
				publish.PivnetProductFilesService = pfs
				publish.PivnetUserGroupsService = ugs
				publish.PivnetReleaseUpgradePathsService = releaseUpgradePathsService
				publish.PivnetReleaseDependenciesService = releaseDependenciesService
				publish.Now = func() time.Time {
					return now
				}
			})

			When("the window flag is not provided", func() {
				BeforeEach(func() {
					executeArgs = []string{"--pivnet-token", "SOME_TOKEN"}
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("missing required flag \"--window\"")))
				})
			})

			When("the an unknown window is provided", func() {
				BeforeEach(func() {
					executeArgs = []string{"--window", "nosuchwindow", "--pivnet-token", "SOME_TOKEN"}
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("unknown window: \"nosuchwindow\"")))
				})
			})

			When("the release is already published", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: "2.8.0"}}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("release 2.8.0 already exists")))
				})
			})

			When("the release to be updated is not found", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: "1.2.3-build.1"}}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("release with version " + someVersion.String() + " not found")))
				})
			})

			When("the version file contains an invalid semver", func() {
				BeforeEach(func() {
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

			When("there is an error fetching product files from Pivnet", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					pfs.ListReturns(nil, errors.New("bad stuff happened"))
				})

				It("returns an error and makes no changes", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())

					Expect(rs.UpdateCallCount()).To(Equal(0))
					Expect(pfs.ListCallCount()).To(Equal(1))
					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
					Expect(err).To(MatchError(ContainSubstring("bad stuff happened")))
				})
			})

			When("there the necessary license file doesn't exist on Pivnet", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					pfs.ListReturns(
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

					Expect(rs.UpdateCallCount()).To(Equal(0))
					Expect(pfs.ListCallCount()).To(Equal(1))
					Expect(pfs.AddToReleaseCallCount()).To(Equal(0))
				})
			})

			When("there is an error adding the license file to the release on Pivnet", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					pfs.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          42,
								Name:        "PCF Pivotal Application Service v2.8 OSL",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
							{
								ID:          43,
								Name:        "PCF Pivotal Application Service v2.8 OSM Part 1/3",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
							{
								ID:          44,
								Name:        "PCF Pivotal Application Service v2.8 OSM Part 2/3",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
							{
								ID:          45,
								Name:        "PCF Pivotal Application Service v2.8 OSM Part 3/3",
								FileVersion: "2.8",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				When("the error occurs before any license files have been attached", func() {
					BeforeEach(func() {
						pfs.AddToReleaseReturns(errors.New("more bad stuff happened"))
					})

					It("returns an error and makes no changes", func() {
						err := publish.Execute(executeArgs)
						Expect(err).To(HaveOccurred())

						Expect(rs.UpdateCallCount()).To(Equal(0))
						Expect(pfs.ListCallCount()).To(Equal(1))
						Expect(pfs.AddToReleaseCallCount()).To(Equal(1))
						Expect(err).To(MatchError(ContainSubstring("failed to publish tile: more bad stuff happened")))
						Expect(errLoggerBuffer.String()).NotTo(ContainSubstring("Attached the following license files before failure:"))
						Expect(errLoggerBuffer.String()).NotTo(ContainSubstring("License file:"))
					})
				})

				When("the error occurs after some license files have already been attached", func() {
					BeforeEach(func() {
						pfs.AddToReleaseReturnsOnCall(2, errors.New("more bad stuff happened"))
					})

					It("returns an error and prints out which product files were already attached", func() {
						err := publish.Execute(executeArgs)
						Expect(err).To(HaveOccurred())

						Expect(rs.UpdateCallCount()).To(Equal(0))
						Expect(pfs.ListCallCount()).To(Equal(1))
						Expect(pfs.AddToReleaseCallCount()).To(Equal(3))
						Expect(err).To(MatchError(ContainSubstring("failed to publish tile: more bad stuff happened")))

						Expect(errLoggerBuffer.String()).To(ContainSubstring("Attached the following license files before failure:"))
						Expect(errLoggerBuffer.String()).To(ContainSubstring("  License file: PCF Pivotal Application Service v2.8 OSL"))
						Expect(errLoggerBuffer.String()).To(ContainSubstring("  License file: PCF Pivotal Application Service v2.8 OSM Part 1/3"))
						Expect(errLoggerBuffer.String()).NotTo(ContainSubstring("2/3"))
						Expect(errLoggerBuffer.String()).NotTo(ContainSubstring("3/3"))
					})
				})
			})

			When("a release on PivNet has an invalid version", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{
						{Version: someVersion.String()},
						{Version: "invalid version"},
					}, nil)
					pfs.ListReturns(
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

				It("ignores that release and updates the correct release", func() {
					err := publish.Execute(executeArgs)
					Expect(err).NotTo(HaveOccurred())

					Expect(rs.UpdateCallCount()).To(Equal(1))
					{
						s, r := rs.UpdateArgsForCall(0)
						Expect(s).To(Equal(slug))
						Expect(r.Version).To(Equal("2.8.0"))
						Expect(r.ReleaseType).To(BeEquivalentTo("Minor Release"))
					}
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Version: 2.8.0"))
					Expect(outLoggerBuffer.String()).To(ContainSubstring("Release type: Minor Release"))
				})
			})

			When("the previous release on PivNet does not have an EOGS date", func() {
				BeforeEach(func() {
					someVersion = semver.MustParse("2.9.1-build.111")

					rs.ListReturns([]pivnet.Release{
						{Version: someVersion.String(), ID: 99},
						{Version: "2.9.0", EndOfSupportDate: ""},
					}, nil)

					pfs.ListReturns(
						[]pivnet.ProductFile{
							{
								ID:          42,
								Name:        "PCF Pivotal Application Service v2.9 OSL",
								FileVersion: "2.9",
								FileType:    "Open Source License",
							},
						},
						nil,
					)
				})

				It("returns an error instead of publishing the release", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("does not have an End of General Support date")))

					Expect(rs.UpdateCallCount()).To(Equal(0))
				})
			})

			When("there is an error fetching user groups from PivNet", func() {
				BeforeEach(func() {
					executeArgs = []string{"--pivnet-token", "SOME_TOKEN", "--window", "rc"}
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					ugs.ListReturns(nil, errors.New("error returning user groups"))
				})

				It("returns an error ", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("error returning user groups")))
				})
			})

			When("there is an error adding a user group to release", func() {
				BeforeEach(func() {
					executeArgs = []string{"--pivnet-token", "SOME_TOKEN", "--window", "rc"}
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					ugs.ListReturns([]pivnet.UserGroup{
						{ID: userGroup1ID, Name: userGroup1Name},
						{ID: userGroup2ID, Name: userGroup2Name},
					}, nil)
					ugs.AddToReleaseReturns(errors.New("error adding user group to release"))
				})

				It("returns an error ", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("error adding user group to release")))
				})
			})

			When("one of the required user groups doesn't exist", func() {
				BeforeEach(func() {
					executeArgs = []string{"--pivnet-token", "SOME_TOKEN", "--window", "rc"}
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					ugs.ListReturns([]pivnet.UserGroup{
						{ID: userGroup2ID, Name: userGroup2Name},
					}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring(userGroup1Name)))
				})
			})

			When("upgrade path is empty", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					pfs.ListReturns(
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
					releaseUpgradePathsService.GetReturns([]pivnet.ReleaseUpgradePath{}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("no upgrade paths set for 2.8")))
				})
			})

			When("dependencies is empty", func() {
				BeforeEach(func() {
					rs.ListReturns([]pivnet.Release{{Version: someVersion.String()}}, nil)
					pfs.ListReturns(
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
					releaseDependenciesService.ListReturns([]pivnet.ReleaseDependency{}, nil)
				})

				It("returns an error", func() {
					err := publish.Execute(executeArgs)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("no dependencies set for 2.8")))
				})
			})
		})
	})
})

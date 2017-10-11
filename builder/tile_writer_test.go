package builder_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"
	"github.com/pivotal-cf/kiln/commands"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("TileWriter", func() {
	var (
		filesystem              *fakes.Filesystem
		zipper                  *fakes.Zipper
		logger                  *fakes.Logger
		contentMigrationBuilder *fakes.ContentMigrationBuilder
		md5Calc                 *fakes.MD5SumCalculator
		tileWriter              builder.TileWriter
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}
		zipper = &fakes.Zipper{}
		logger = &fakes.Logger{}
		md5Calc = &fakes.MD5SumCalculator{}
		contentMigrationBuilder = &fakes.ContentMigrationBuilder{}
		tileWriter = builder.NewTileWriter(filesystem, zipper, contentMigrationBuilder, logger, md5Calc)
	})

	Describe("Build", func() {
		DescribeTable("writes tile to disk", func(stubbed bool, release1Content, release2Content string, errorWhenAttemptingToOpenRelease error) {
			config := commands.BakeConfig{
				ProductName:          "cool-product-name",
				FilenamePrefix:       "cool-product-file",
				ReleasesDirectory:    "/some/path/releases",
				MigrationDirectories: []string{"/some/path/migrations", "/some/other/path/migrations"},
				ContentMigrations:    []string{"/some/path/content-migration-1.yml", "/some/path/content-migration-2.yml"},
				BaseContentMigration: "/some/path/base-content-migration.yml",
				Version:              "1.2.3-build.4",
				FinalVersion:         "1.2.3",
				StubReleases:         stubbed,
			}

			contentMigrationBuilder.BuildCall.Returns.ContentMigration = []byte("combined-content-migration-contents")

			dirInfo := &fakes.FileInfo{}
			dirInfo.IsDirReturns(true)

			releaseInfo := &fakes.FileInfo{}
			releaseInfo.IsDirReturns(false)

			migrationInfo := &fakes.FileInfo{}
			migrationInfo.IsDirReturns(false)

			filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
				switch root {
				case "/some/path/releases":
					walkFn("/some/path/releases", dirInfo, nil)
					walkFn("/some/path/releases/release-1.tgz", releaseInfo, nil)
					walkFn("/some/path/releases/release-2.tgz", releaseInfo, nil)
				case "/some/path/migrations":
					walkFn("/some/path/migrations", dirInfo, nil)
					walkFn("/some/path/migrations/migration-1.js", migrationInfo, nil)
					walkFn("/some/path/migrations/migration-2.js", migrationInfo, nil)
				case "/some/other/path/migrations":
					walkFn("/some/other/path/migrations", dirInfo, nil)
					walkFn("/some/other/path/migrations/other-migration.js", migrationInfo, nil)
				default:
					return nil
				}
				return nil
			}

			filesystem.OpenStub = func(path string) (io.ReadWriteCloser, error) {
				switch path {
				case "/some/path/releases/release-1.tgz":
					return NewBuffer(bytes.NewBuffer([]byte("release-1"))), errorWhenAttemptingToOpenRelease
				case "/some/path/releases/release-2.tgz":
					return NewBuffer(bytes.NewBuffer([]byte("release-2"))), errorWhenAttemptingToOpenRelease
				case "/some/path/migrations/migration-1.js":
					return NewBuffer(bytes.NewBuffer([]byte("migration-1"))), nil
				case "/some/path/migrations/migration-2.js":
					return NewBuffer(bytes.NewBuffer([]byte("migration-2"))), nil
				case "/some/other/path/migrations/other-migration.js":
					return NewBuffer(bytes.NewBuffer([]byte("other-migration"))), nil
				default:
					return nil, fmt.Errorf("open %s: no such file or directory", path)
				}
			}

			md5Calc.ChecksumCall.Returns.Sum = "THIS-IS-THE-SUM"

			err := tileWriter.Write([]byte("metadata-contents"), config)
			Expect(err).NotTo(HaveOccurred())

			Expect(contentMigrationBuilder.BuildCall.CallCount).To(Equal(1))
			Expect(contentMigrationBuilder.BuildCall.Receives.BaseContentMigration).To(Equal("/some/path/base-content-migration.yml"))
			Expect(contentMigrationBuilder.BuildCall.Receives.ContentMigrations).To(Equal([]string{"/some/path/content-migration-1.yml", "/some/path/content-migration-2.yml"}))
			Expect(contentMigrationBuilder.BuildCall.Receives.Version).To(Equal("1.2.3"))

			Expect(zipper.SetPathCall.CallCount).To(Equal(1))
			Expect(zipper.SetPathCall.Receives.Path).To(Equal("cool-product-file-1.2.3-build.4.pivotal"))

			Expect(zipper.AddCall.Calls).To(HaveLen(7))

			Expect(zipper.AddCall.Calls[0].Path).To(Equal(filepath.Join("content_migrations", "migrations.yml")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[0].File)).Should(gbytes.Say("combined-content-migration-contents"))

			Expect(zipper.AddCall.Calls[1].Path).To(Equal(filepath.Join("metadata", "cool-product-name.yml")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[1].File)).Should(gbytes.Say("metadata-contents"))

			Expect(zipper.AddCall.Calls[2].Path).To(Equal(filepath.Join("migrations", "v1", "migration-1.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[2].File)).Should(gbytes.Say("migration-1"))

			Expect(zipper.AddCall.Calls[3].Path).To(Equal(filepath.Join("migrations", "v1", "migration-2.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[3].File)).Should(gbytes.Say("migration-2"))

			Expect(zipper.AddCall.Calls[4].Path).To(Equal(filepath.Join("migrations", "v1", "other-migration.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[4].File)).Should(gbytes.Say("other-migration"))

			Expect(zipper.AddCall.Calls[5].Path).To(Equal(filepath.Join("releases", "release-1.tgz")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[5].File)).Should(gbytes.Say(release1Content))

			Expect(zipper.AddCall.Calls[6].Path).To(Equal(filepath.Join("releases", "release-2.tgz")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[6].File)).Should(gbytes.Say(release2Content))

			Expect(zipper.CloseCall.CallCount).To(Equal(1))

			Expect(logger.PrintlnCall.Receives.LogLines).To(Equal([]string{"Building .pivotal file...", "Calculating md5 sum of .pivotal..."}))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				"Adding content_migrations/migrations.yml to .pivotal...",
				"Adding metadata/cool-product-name.yml to .pivotal...",
				"Adding migrations/v1/migration-1.js to .pivotal...",
				"Adding migrations/v1/migration-2.js to .pivotal...",
				"Adding migrations/v1/other-migration.js to .pivotal...",
				"Adding releases/release-1.tgz to .pivotal...",
				"Adding releases/release-2.tgz to .pivotal...",
				"Calculated md5 sum: THIS-IS-THE-SUM",
			}))

			Expect(md5Calc.ChecksumCall.CallCount).To(Equal(1))
			Expect(md5Calc.ChecksumCall.Receives.Path).To(Equal("cool-product-file-1.2.3-build.4.pivotal"))
		},
			Entry("without stubbing releases", false, "release-1", "release-2", nil),
			Entry("with stubbed releases", true, "", "", errors.New("don't open release")),
		)

		Context("when releases directory is provided", func() {
			BeforeEach(func() {
				dirInfo := &fakes.FileInfo{}
				dirInfo.IsDirReturns(true)

				releaseInfo := &fakes.FileInfo{}
				releaseInfo.IsDirReturns(false)

				filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
					switch root {
					case "/some/path/releases":
						walkFn("/some/path/releases", dirInfo, nil)
						walkFn("/some/path/releases/release-1.tgz", releaseInfo, nil)
						walkFn("/some/path/releases/release-2.tgz", releaseInfo, nil)
						walkFn(root, dirInfo, nil)
					case "/some/path/migrations":
						walkFn("/some/path/migrations", dirInfo, nil)
					default:
						return nil
					}

					return nil
				}

				filesystem.OpenStub = func(path string) (io.ReadWriteCloser, error) {
					if path == "/some/path/releases/release-1.tgz" {
						return NewBuffer(bytes.NewBufferString("release-1")), nil
					}

					if path == "/some/path/releases/release-2.tgz" {
						return NewBuffer(bytes.NewBufferString("release-1")), nil
					}

					return nil, nil
				}
			})

			Context("and no migrations are provided", func() {
				It("creates empty migrations/v1 folder", func() {
					config := commands.BakeConfig{
						ProductName:          "cool-product-name",
						FilenamePrefix:       "cool-product-file",
						ReleasesDirectory:    "/some/path/releases",
						MigrationDirectories: []string{},
						ContentMigrations:    []string{},
						BaseContentMigration: "",
						Version:              "1.2.3-build.4",
						FinalVersion:         "1.2.3",
						StubReleases:         false,
					}

					err := tileWriter.Write([]byte("metadata-contents"), config)
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
						"Creating empty migrations folder in .pivotal...",
						"Adding metadata/cool-product-name.yml to .pivotal...",
						"Adding releases/release-1.tgz to .pivotal...",
						"Adding releases/release-2.tgz to .pivotal...",
						"Calculated md5 sum: ",
					}))
					Expect(zipper.CreateFolderCall.CallCount).To(Equal(1))
					Expect(zipper.CreateFolderCall.Receives.Path).To(Equal(filepath.Join("migrations", "v1")))
				})
			})

			Context("and the migrations directory is empty", func() {
				It("creates empty migrations/v1 folder", func() {
					config := commands.BakeConfig{
						ProductName:          "cool-product-name",
						FilenamePrefix:       "cool-product-file",
						ReleasesDirectory:    "/some/path/releases",
						MigrationDirectories: []string{"/some/path/migrations"},
						ContentMigrations:    []string{},
						BaseContentMigration: "",
						Version:              "1.2.3-build.4",
						FinalVersion:         "1.2.3",
						StubReleases:         false,
					}

					err := tileWriter.Write([]byte("metadata-contents"), config)
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
						"Creating empty migrations folder in .pivotal...",
						"Adding metadata/cool-product-name.yml to .pivotal...",
						"Adding releases/release-1.tgz to .pivotal...",
						"Adding releases/release-2.tgz to .pivotal...",
						"Calculated md5 sum: ",
					}))
					Expect(zipper.CreateFolderCall.CallCount).To(Equal(1))
					Expect(zipper.CreateFolderCall.Receives.Path).To(Equal(filepath.Join("migrations", "v1")))
				})
			})
		})

		Context("failure cases", func() {
			Context("when the zipper fails to create migrations folder", func() {
				It("returns an error", func() {
					config := commands.BakeConfig{
						StubReleases: true,
					}

					zipper.CreateFolderCall.Returns.Error = errors.New("failed to create folder")
					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError("failed to create folder"))
				})
			})

			Context("when a release file cannot be opened", func() {
				It("returns an error", func() {
					dirInfo := &fakes.FileInfo{}
					dirInfo.IsDirReturns(true)

					releaseInfo := &fakes.FileInfo{}
					releaseInfo.IsDirReturns(false)

					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						walkFn("/some/path/releases", dirInfo, nil)
						err := walkFn("/some/path/releases/release-1.tgz", releaseInfo, nil)

						return err
					}

					filesystem.OpenStub = func(path string) (io.ReadWriteCloser, error) {
						if path == "/some/path/releases/release-1.tgz" {
							return nil, errors.New("failed to open release")
						}

						return nil, nil
					}

					config := commands.BakeConfig{
						ReleasesDirectory: "/some/path/releases",
					}

					err := tileWriter.Write([]byte("metadata-contents"), config)
					Expect(err).To(MatchError("failed to open release"))
				})
			})

			Context("when a migration file cannot be opened", func() {
				It("returns an error", func() {
					dirInfo := &fakes.FileInfo{}
					dirInfo.IsDirReturns(true)

					releaseInfo := &fakes.FileInfo{}
					releaseInfo.IsDirReturns(false)

					migrationInfo := &fakes.FileInfo{}
					migrationInfo.IsDirReturns(false)

					filesystem.WalkStub = func(root string, walkFn filepath.WalkFunc) error {
						walkFn("/some/path/migrations", dirInfo, nil)
						err := walkFn("/some/path/migrations/migration-1.js", migrationInfo, nil)
						if err != nil {
							return err
						}

						walkFn("/some/path/releases", dirInfo, nil)
						err = walkFn("/some/path/releases/release-1.tgz", releaseInfo, nil)

						return err
					}

					filesystem.OpenStub = func(path string) (io.ReadWriteCloser, error) {
						if path == "/some/path/migrations/migration-1.js" {
							return nil, errors.New("failed to open migration")
						}

						if path == "/some/path/releases/release-1.tgz" {
							return NewBuffer(bytes.NewBufferString("release-1")), nil
						}

						return nil, nil
					}

					config := commands.BakeConfig{
						ReleasesDirectory:    "/some/path/releases",
						MigrationDirectories: []string{"/some/path/migrations"},
						StubReleases:         true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("failed to open migration"))
				})
			})

			Context("when the zipper fails to add a file", func() {
				It("returns an error", func() {
					zipper.AddCall.Returns.Error = errors.New("failed to add file to zip")

					config := commands.BakeConfig{
						StubReleases: true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("failed to add file to zip"))
				})
			})

			Context("when the zipper fails to close", func() {
				It("returns an error", func() {
					zipper.CloseCall.Returns.Error = errors.New("failed to close the zip")

					config := commands.BakeConfig{
						StubReleases: true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("failed to close the zip"))
				})
			})

			Context("when content migration builder fails", func() {
				It("returns an error", func() {
					contentMigrationBuilder.BuildCall.Returns.Error = errors.New("builder failed")

					config := commands.BakeConfig{
						ContentMigrations:    []string{"some-migration-file.yml"},
						BaseContentMigration: "base-migration-file.yml",
						StubReleases:         true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("builder failed"))
				})
			})

			Context("when setting the path on the zipper fails", func() {
				It("returns an error", func() {
					zipper.SetPathCall.Returns.Error = errors.New("zipper set path failed")

					config := commands.BakeConfig{
						StubReleases: true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("zipper set path failed"))
				})
			})
			Context("when the MD5 cannot be calculated", func() {
				It("returns an error", func() {

					md5Calc.ChecksumCall.Returns.Error = errors.New("MD5 cannot be calculated")

					config := commands.BakeConfig{
						StubReleases: true,
					}

					err := tileWriter.Write([]byte{}, config)
					Expect(err).To(MatchError("MD5 cannot be calculated"))
				})
			})

		})
	})
})

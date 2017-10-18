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
		filesystem *fakes.Filesystem
		zipper     *fakes.Zipper
		logger     *fakes.Logger
		md5Calc    *fakes.MD5SumCalculator
		tileWriter builder.TileWriter
		outputFile string
	)

	BeforeEach(func() {
		filesystem = &fakes.Filesystem{}
		zipper = &fakes.Zipper{}
		logger = &fakes.Logger{}
		md5Calc = &fakes.MD5SumCalculator{}
		tileWriter = builder.NewTileWriter(filesystem, zipper, logger, md5Calc)
		outputFile = "some-output-dir/cool-product-file-1.2.3-build.4.pivotal"
	})

	Describe("Build", func() {
		DescribeTable("writes tile to disk", func(stubbed bool, errorWhenAttemptingToOpenRelease error) {
			config := commands.BakeConfig{
				ProductName:          "cool-product-name",
				ReleaseDirectories:   []string{"/some/path/releases", "/some/other/path/releases"},
				MigrationDirectories: []string{"/some/path/migrations", "/some/other/path/migrations"},
				Version:              "1.2.3",
				OutputFile:           outputFile,
				StubReleases:         stubbed,
			}

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
				case "/some/other/path/releases":
					walkFn("/some/other/path/releases", dirInfo, nil)
					walkFn("/some/other/path/releases/release-3.tgz", releaseInfo, nil)
					walkFn("/some/other/path/releases/release-4.tgz", releaseInfo, nil)
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
				case "/some/other/path/releases/release-3.tgz":
					return NewBuffer(bytes.NewBuffer([]byte("release-3"))), errorWhenAttemptingToOpenRelease
				case "/some/other/path/releases/release-4.tgz":
					return NewBuffer(bytes.NewBuffer([]byte("release-4"))), errorWhenAttemptingToOpenRelease
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

			Expect(zipper.SetPathCall.CallCount).To(Equal(1))
			Expect(zipper.SetPathCall.Receives.Path).To(Equal("some-output-dir/cool-product-file-1.2.3-build.4.pivotal"))

			Expect(zipper.AddCall.Calls).To(HaveLen(8))

			Expect(zipper.AddCall.Calls[0].Path).To(Equal(filepath.Join("metadata", "cool-product-name.yml")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[0].File)).Should(gbytes.Say("metadata-contents"))

			Expect(zipper.AddCall.Calls[1].Path).To(Equal(filepath.Join("migrations", "v1", "migration-1.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[1].File)).Should(gbytes.Say("migration-1"))

			Expect(zipper.AddCall.Calls[2].Path).To(Equal(filepath.Join("migrations", "v1", "migration-2.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[2].File)).Should(gbytes.Say("migration-2"))

			Expect(zipper.AddCall.Calls[3].Path).To(Equal(filepath.Join("migrations", "v1", "other-migration.js")))
			Eventually(gbytes.BufferReader(zipper.AddCall.Calls[3].File)).Should(gbytes.Say("other-migration"))

			Expect(zipper.AddCall.Calls[4].Path).To(Equal(filepath.Join("releases", "release-1.tgz")))
			checkReleaseFileContent("release-1", stubbed, zipper.AddCall.Calls[4])

			Expect(zipper.AddCall.Calls[5].Path).To(Equal(filepath.Join("releases", "release-2.tgz")))
			checkReleaseFileContent("release-2", stubbed, zipper.AddCall.Calls[5])

			Expect(zipper.AddCall.Calls[6].Path).To(Equal(filepath.Join("releases", "release-3.tgz")))
			checkReleaseFileContent("release-3", stubbed, zipper.AddCall.Calls[6])

			Expect(zipper.AddCall.Calls[7].Path).To(Equal(filepath.Join("releases", "release-4.tgz")))
			checkReleaseFileContent("release-4", stubbed, zipper.AddCall.Calls[7])

			Expect(zipper.CloseCall.CallCount).To(Equal(1))

			Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
				fmt.Sprintf("Building %s...", outputFile),
				fmt.Sprintf("Adding metadata/cool-product-name.yml to %s...", outputFile),
				fmt.Sprintf("Adding migrations/v1/migration-1.js to %s...", outputFile),
				fmt.Sprintf("Adding migrations/v1/migration-2.js to %s...", outputFile),
				fmt.Sprintf("Adding migrations/v1/other-migration.js to %s...", outputFile),
				fmt.Sprintf("Adding releases/release-1.tgz to %s...", outputFile),
				fmt.Sprintf("Adding releases/release-2.tgz to %s...", outputFile),
				fmt.Sprintf("Adding releases/release-3.tgz to %s...", outputFile),
				fmt.Sprintf("Adding releases/release-4.tgz to %s...", outputFile),
				fmt.Sprintf("Calculating md5 sum of %s...", outputFile),
				"Calculated md5 sum: THIS-IS-THE-SUM",
			}))

			Expect(md5Calc.ChecksumCall.CallCount).To(Equal(1))
			Expect(md5Calc.ChecksumCall.Receives.Path).To(Equal("some-output-dir/cool-product-file-1.2.3-build.4.pivotal"))
		},
			Entry("without stubbing releases", false, nil),
			Entry("with stubbed releases", true, errors.New("don't open release")),
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
						ReleaseDirectories:   []string{"/some/path/releases"},
						MigrationDirectories: []string{},
						Version:              "1.2.3",
						OutputFile:           "some-output-dir/cool-product-file-1.2.3-build.4.pivotal",
						StubReleases:         false,
					}

					err := tileWriter.Write([]byte("metadata-contents"), config)
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
						fmt.Sprintf("Building %s...", outputFile),
						fmt.Sprintf("Creating empty migrations folder in %s...", outputFile),
						fmt.Sprintf("Adding metadata/cool-product-name.yml to %s...", outputFile),
						fmt.Sprintf("Adding releases/release-1.tgz to %s...", outputFile),
						fmt.Sprintf("Adding releases/release-2.tgz to %s...", outputFile),
						fmt.Sprintf("Calculating md5 sum of %s...", outputFile),
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
						ReleaseDirectories:   []string{"/some/path/releases"},
						MigrationDirectories: []string{"/some/path/migrations"},
						Version:              "1.2.3",
						OutputFile:           "some-output-dir/cool-product-file-1.2.3-build.4.pivotal",
						StubReleases:         false,
					}

					err := tileWriter.Write([]byte("metadata-contents"), config)
					Expect(err).NotTo(HaveOccurred())

					Expect(logger.PrintfCall.Receives.LogLines).To(Equal([]string{
						fmt.Sprintf("Building %s...", outputFile),
						fmt.Sprintf("Creating empty migrations folder in %s...", outputFile),
						fmt.Sprintf("Adding metadata/cool-product-name.yml to %s...", outputFile),
						fmt.Sprintf("Adding releases/release-1.tgz to %s...", outputFile),
						fmt.Sprintf("Adding releases/release-2.tgz to %s...", outputFile),
						fmt.Sprintf("Calculating md5 sum of %s...", outputFile),
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
						ReleaseDirectories: []string{"/some/path/releases"},
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
						ReleaseDirectories:   []string{"/some/path/releases"},
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

func checkReleaseFileContent(releaseContent string, stubbed bool, call fakes.ZipperAddCall) {
	if stubbed == false {
		Eventually(gbytes.BufferReader(call.File)).Should(gbytes.Say(releaseContent))
	} else {
		Eventually(gbytes.BufferReader(call.File)).Should(gbytes.Say(""))
	}
}

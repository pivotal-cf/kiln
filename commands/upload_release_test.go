package commands_test

import (
	"io/ioutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/commands"
	"github.com/pivotal-cf/kiln/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/cargo"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
)

var _ = Describe("UploadRelease", func() {
	Context("Execute", func() {
		var (
			fs       billy.Filesystem
			loader   *fakes.KilnfileLoader
			uploader *fakes.S3Uploader

			uploadRelease commands.UploadRelease

			exampleReleaseSourceList = func() []cargo.ReleaseSourceConfig {
				return []cargo.ReleaseSourceConfig{
					{
						Type:            "s3",
						Bucket:          "orange-bucket",
						Region:          "mars-2",
						AccessKeyId:     "id",
						SecretAccessKey: "secret",
						Regex:           "^(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\\.]+-?[a-zA-Z0-9]\\.?[0-9]*)\\.tgz$",
					},
					{
						Type: "boshio",
					},
					{
						Type:            "s3",
						Bucket:          "lemon-bucket",
						Region:          "mars-2",
						AccessKeyId:     "id",
						SecretAccessKey: "secret",
					},
				}
			}
		)

		BeforeEach(func() {
			fs = memfs.New()
			loader = new(fakes.KilnfileLoader)
			uploader = new(fakes.S3Uploader)

			uploadRelease = commands.UploadRelease{
				FS:             fs,
				KilnfileLoader: loader,
				UploaderConfig: func(rsc *cargo.ReleaseSourceConfig) (commands.S3Uploader, error) {
					Fail("this function should be overridden in tests that use it")
					return nil, nil
				},
			}
		})

		When("it receives a correct tarball path", func() {
			BeforeEach(func() {
				loader.LoadKilnfilesReturns(
					cargo.Kilnfile{ReleaseSources: exampleReleaseSourceList()},
					cargo.KilnfileLock{}, nil)

				f, err := fs.Create("banana-release.tgz")
				_, _ = f.Write([]byte("banana"))
				f.Close()

				Expect(err).NotTo(HaveOccurred())
			})

			It("uploads the tarball to the release source", func() {
				configUploaderCallCount := 0

				var relSrcConfig *cargo.ReleaseSourceConfig

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) (commands.S3Uploader, error) {
					configUploaderCallCount++
					relSrcConfig = rsc
					return uploader, nil
				}

				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--remote-path", "banana-release-1.2.3.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(configUploaderCallCount).To(Equal(1))
				Expect(relSrcConfig).NotTo(BeNil())
				Expect(relSrcConfig.Bucket).To(Equal("orange-bucket"))
				Expect(uploader.UploadCallCount()).To(Equal(1))

				opts, fns := uploader.UploadArgsForCall(0)

				Expect(fns).To(HaveLen(0))

				Expect(opts.Bucket).NotTo(BeNil())
				Expect(*opts.Bucket).To(Equal("orange-bucket"))
				Expect(opts.Key).NotTo(BeNil())
				Expect(*opts.Key).To(Equal("banana-release-1.2.3.tgz"))

				buf, _ := ioutil.ReadAll(opts.Body)
				Expect(string(buf)).To(Equal("banana"))
			})
		})

		When("the release source in Kilnfile has an invalid regular expression", func() {
			BeforeEach(func() {
				relSrcList := exampleReleaseSourceList()

				relSrcList[0].Regex = "^(?P<bad_regex"

				loader.LoadKilnfilesReturns(cargo.Kilnfile{
					ReleaseSources: relSrcList,
				}, cargo.KilnfileLock{}, nil)

				f, err := fs.Create("banana-release.tgz")
				_, _ = f.Write([]byte("banana"))
				f.Close()

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) (commands.S3Uploader, error) {
					return uploader, nil
				}

				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a descriptive error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--remote-path", "banana-release-1.2.3.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).To(MatchError(ContainSubstring("could not compile the regular expression")))
			})
		})

		When("the remote-path does not match the regex in the release_source", func() {
			BeforeEach(func() {
				relSrcList := exampleReleaseSourceList()

				loader.LoadKilnfilesReturns(cargo.Kilnfile{
					ReleaseSources: relSrcList,
				}, cargo.KilnfileLock{}, nil)

				f, err := fs.Create("banana-release.tgz")
				_, _ = f.Write([]byte("banana"))
				f.Close()

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) (commands.S3Uploader, error) {
					return uploader, nil
				}

				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a descriptive error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--remote-path", "BLA_BLA_BLA.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).To(MatchError(ContainSubstring("remote-path does not match")))
			})
		})

		When("some the remote does not exist in the Kilnfile", func() {
			When("no release sources are s3 buckets", func() {
				BeforeEach(func() {
					loader.LoadKilnfilesReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, nil)

					f, err := fs.Create("banana-release.tgz")
					_, _ = f.Write([]byte("banana"))
					f.Close()

					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error without suggested release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--remote-path", "banana-release-1.2.3.tgz",
						"--release-source", "orange-bucket",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("remote release source")))
				})
			})
			When("at least one release source is an s3 bucket", func() {
				BeforeEach(func() {
					loader.LoadKilnfilesReturns(
						cargo.Kilnfile{ReleaseSources: exampleReleaseSourceList()},
						cargo.KilnfileLock{}, nil,
					)

					f, err := fs.Create("banana-release.tgz")
					_, _ = f.Write([]byte("banana"))
					f.Close()

					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error without suggested release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--remote-path", "banana-release-1.2.3.tgz",
						"--release-source", "dog-bucket",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("orange-bucket")))
				})
			})
		})
	})
})

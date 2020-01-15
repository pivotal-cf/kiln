package commands_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

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
						Regex:           `^\w+/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\.]+-?[a-zA-Z0-9]\.?[0-9]*)\.tgz$`,
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

			expectedReleaseSHA string
		)

		var writeReleaseTarball = func(path, name, version string) string {
			f, err := fs.Create(path)
			Expect(err).NotTo(HaveOccurred())

			gw := gzip.NewWriter(f)
			tw := tar.NewWriter(gw)

			releaseManifest := `
name: ` + name + `
version: ` + version + `
`
			manifestReader := strings.NewReader(releaseManifest)

			header := &tar.Header{
				Name:    "release.MF",
				Size:    manifestReader.Size(),
				Mode:    int64(os.O_RDONLY),
				ModTime: time.Now(),
			}
			Expect(tw.WriteHeader(header)).To(Succeed())

			_, err = io.Copy(tw, manifestReader)
			Expect(err).NotTo(HaveOccurred())

			Expect(tw.Close()).To(Succeed())
			Expect(gw.Close()).To(Succeed())
			Expect(f.Close()).To(Succeed())

			tarball, err := fs.Open(path)
			Expect(err).NotTo(HaveOccurred())
			defer tarball.Close()

			hash := sha1.New()
			_, err = io.Copy(hash, tarball)
			Expect(err).NotTo(HaveOccurred())

			return fmt.Sprintf("%x", hash.Sum(nil))
		}

		BeforeEach(func() {
			fs = memfs.New()
			loader = new(fakes.KilnfileLoader)
			uploader = new(fakes.S3Uploader)

			uploadRelease = commands.UploadRelease{
				FS:             fs,
				KilnfileLoader: loader,
				Logger:         log.New(GinkgoWriter, "", 0),
				UploaderConfig: func(rsc *cargo.ReleaseSourceConfig) commands.S3Uploader {
					Fail("this function should be overridden in tests that use it")
					return nil
				},
			}
			expectedReleaseSHA = writeReleaseTarball("banana-release.tgz", "banana", "1.2.3")
		})

		When("it receives a correct tarball path", func() {
			BeforeEach(func() {
				loader.LoadKilnfilesReturns(
					cargo.Kilnfile{ReleaseSources: exampleReleaseSourceList()},
					cargo.KilnfileLock{}, nil)
			})

			It("uploads the tarball to the release source", func() {
				configUploaderCallCount := 0

				var relSrcConfig *cargo.ReleaseSourceConfig

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) commands.S3Uploader {
					configUploaderCallCount++
					relSrcConfig = rsc
					return uploader
				}

				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
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
				Expect(*opts.Key).To(Equal("banana/banana-1.2.3.tgz"))

				hash := sha1.New()
				_, err = io.Copy(hash, opts.Body)
				Expect(err).NotTo(HaveOccurred())

				releaseSHA := fmt.Sprintf("%x", hash.Sum(nil))
				Expect(releaseSHA).To(Equal(expectedReleaseSHA))
			})
		})

		When("the release tarball is invalid", func() {
			BeforeEach(func() {
				f, err := fs.Create("invalid-release.tgz")
				_, _ = f.Write([]byte("invalid"))
				f.Close()

				Expect(err).NotTo(HaveOccurred())

				loader.LoadKilnfilesReturns(
					cargo.Kilnfile{ReleaseSources: exampleReleaseSourceList()},
					cargo.KilnfileLock{}, nil)
			})

			It("errors", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "invalid-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})
				Expect(err).To(MatchError(ContainSubstring("error reading the release manifest")))
			})
		})

		When("the release source in Kilnfile has an invalid regular expression", func() {
			BeforeEach(func() {
				relSrcList := exampleReleaseSourceList()

				relSrcList[0].Regex = "^(?P<bad_regex"

				loader.LoadKilnfilesReturns(cargo.Kilnfile{
					ReleaseSources: relSrcList,
				}, cargo.KilnfileLock{}, nil)

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) commands.S3Uploader {
					return uploader
				}
			})

			It("returns a descriptive error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).To(MatchError(ContainSubstring("could not compile the regular expression")))
			})
		})

		When("the conventional remote-path does not match the regex in the release_source", func() {
			BeforeEach(func() {
				relSrcList := []cargo.ReleaseSourceConfig{
					{
						Type:            "s3",
						Bucket:          "orange-bucket",
						Region:          "mars-2",
						AccessKeyId:     "id",
						SecretAccessKey: "secret",
						Regex:           "^pointless-root-dir/(?P<release_name>[a-z-_0-9]+)-(?P<release_version>v?[0-9\\.]+-?[a-zA-Z0-9]\\.?[0-9]*)\\.tgz$",
					},
				}

				loader.LoadKilnfilesReturns(cargo.Kilnfile{
					ReleaseSources: relSrcList,
				}, cargo.KilnfileLock{}, nil)

				uploadRelease.UploaderConfig = func(rsc *cargo.ReleaseSourceConfig) commands.S3Uploader {
					return uploader
				}
			})

			It("returns a descriptive error", func() {
				err := uploadRelease.Execute([]string{
					"--kilnfile", "not-read-see-struct/Kilnfile",
					"--local-path", "banana-release.tgz",
					"--release-source", "orange-bucket",
					"--variables-file", "my-secrets",
				})

				Expect(err).To(MatchError(ContainSubstring(`remote path "banana/banana-1.2.3.tgz" does not match`)))
			})
		})

		When("the given release source doesn't exist", func() {
			When("no release sources are s3 buckets", func() {
				BeforeEach(func() {
					loader.LoadKilnfilesReturns(cargo.Kilnfile{}, cargo.KilnfileLock{}, nil)
				})

				It("returns an error without suggested release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--release-source", "orange-bucket",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("remote release source")))
				})
			})

			When("some release sources are s3 buckets", func() {
				BeforeEach(func() {
					loader.LoadKilnfilesReturns(
						cargo.Kilnfile{ReleaseSources: exampleReleaseSourceList()},
						cargo.KilnfileLock{}, nil,
					)
				})

				It("returns an error that suggests valid release sources", func() {
					err := uploadRelease.Execute([]string{
						"--kilnfile", "not-read-see-struct/Kilnfile",
						"--local-path", "banana-release.tgz",
						"--release-source", "no-such-release-source",
						"--variables-file", "my-secrets",
					})

					Expect(err).To(MatchError(ContainSubstring("orange-bucket")))
				})
			})
		})
	})
})

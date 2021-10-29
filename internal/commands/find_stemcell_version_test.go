package commands_test

import (
	"errors"
	"log"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/options"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("Find the stemcell version", func() {
	var (
		findStemcellVersion commands.FindStemcellVersion
		logger              *log.Logger

		writer strings.Builder

		kf       cargo.Kilnfile
		kl       cargo.KilnfileLock
		parseErr error

		pivnet        component.Pivnet
		serverMock    *fakes.RoundTripper
		simpleRequest *http.Request
		requestErr    error
		executeErr    error
	)

	Describe("Execute", func() {
		BeforeEach(func() {
			logger = log.New(&writer, "", 0)

			pivnet = component.Pivnet{}
			simpleRequest, _ = http.NewRequest(http.MethodGet, "/", nil)

			serverMock = &fakes.RoundTripper{}
			serverMock.Results.Res = &http.Response{}
			pivnet.Client = &http.Client{
				Transport: serverMock,
			}

			kf = cargo.Kilnfile{
				ReleaseSources: []cargo.ReleaseSourceConfig{
					{
						Type:            "s3",
						Bucket:          "compiled-releases",
						Region:          "us-west-1",
						AccessKeyId:     "my-access-key-id",
						SecretAccessKey: "secret_access_key",
						PathTemplate:    `2.8/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz`,
						Publishable:     true,
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "ubuntu-xenial",
					Version: "~456",
				},
			}

			kl = cargo.KilnfileLock{
				Releases: []cargo.ComponentLock{
					{
						Name:         "some-release",
						Version:      "1.2.3",
						RemoteSource: "compiled-releases",
						RemotePath:   "my-remote-path",
					},
				},
				Stemcell: cargo.Stemcell{
					OS:      "some-os",
					Version: "4.5.6",
				},
			}

			parseErr = nil
		})

		JustBeforeEach(func() {
			_, requestErr = pivnet.Do(simpleRequest)
			Expect(requestErr).NotTo(HaveOccurred())

			findStemcellVersion = commands.NewFindStemcellVersion(logger, &pivnet)

			executeErr = findStemcellVersion.KilnExecute(nil,
				func(args []string, ops options.StandardOptionsEmbedder) (cargo.Kilnfile, cargo.KilnfileLock, []string, error) {
					return kf, kl, nil, parseErr
				},
			)
		})

		When("parsing fails", func() {
			BeforeEach(func() {
				parseErr = errors.New("banana")
			})
			It("returns the stemcell os info missing error message", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr).To(MatchError(ContainSubstring("banana")))
			})
		})

		When("stemcell criteria does not exist in the kilnfile", func() {
			BeforeEach(func() {
				kf.Stemcell = cargo.Stemcell{}
			})
			It("returns the stemcell os info missing error message", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr).To(MatchError(ContainSubstring(commands.ErrStemcellOSInfoMustBeValid)))
			})
		})

		When("stemcell major version does not exist in the kilnfile", func() {
			BeforeEach(func() {
				kf.Stemcell.Version = ""
			})

			It("returns stemcell major version missing error message", func() {
				Expect(executeErr).To(HaveOccurred())
				Expect(executeErr).To(MatchError(ContainSubstring(commands.ErrStemcellMajorVersionMustBeValid)))
			})
		})

		When("stemcell OS and major version is specified", func() {
			When("a new stemcell exists", func() {
				BeforeEach(func() {
					serverMock.Results.Res.Body = fakes.NewReadCloser(`{"version": "456.118"}`)
					serverMock.Results.Res.StatusCode = http.StatusOK
					serverMock.Results.Err = nil
				})

				It("returns the latest stemcell version", func() {
					Expect(executeErr).NotTo(HaveOccurred())
					Expect((&writer).String()).To(ContainSubstring("\"456.118\""))
					Expect((&writer).String()).To(ContainSubstring("\"remote_path\":\"network.pivotal.io\""))
					Expect((&writer).String()).To(ContainSubstring("\"source\":\"Tanzunet\""))
				})
			})
		})
	})

	Describe("ExtractMajorVersion", func() {
		var (
			stemcellVersionSpecifier string
			majorVersion             string
			err                      error
		)

		BeforeEach(func() {
			stemcellVersionSpecifier = "~456"
		})

		JustBeforeEach(func() {
			majorVersion, err = commands.ExtractMajorVersion(stemcellVersionSpecifier)
		})

		When("Invalid Stemcell Version Specifier is provided", func() {
			When("with just *", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "*"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(commands.ErrStemcellMajorVersionMustBeValid))
				})
			})
		})

		When("Valid Stemcell Version Specifier is provided", func() {
			When("with tilde ~ ", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "~456"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(majorVersion).To(Equal("456"))
				})
			})
			When("with hypens -", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "777.1-621"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(majorVersion).To(Equal("777"))
				})
			})

			When("with wildcards *", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "1234.*"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(majorVersion).To(Equal("1234"))
				})
			})

			When("with caret ^", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "^456"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(majorVersion).To(Equal("456"))
				})
			})

			When("with absolute value", func() {
				BeforeEach(func() {
					stemcellVersionSpecifier = "333.334"
				})

				It("returns the latest stemcell version", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(majorVersion).To(Equal("333"))
				})
			})
		})
	})
})

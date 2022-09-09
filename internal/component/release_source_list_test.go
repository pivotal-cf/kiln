package component_test

import (
	"context"
	"errors"
	"gopkg.in/yaml.v3"
	"log"
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/internal/component/fakes"
)

var _ = Describe(reflect.TypeOf(component.ReleaseSources{}).Name(), func() {
	var (
		multiSrc         *component.ReleaseSources
		src1, src2, src3 *fakes.ReleaseSource
		requirement      component.Spec

		ctx    context.Context
		logger *log.Logger
	)

	const (
		releaseName         = "stuff-and-things"
		releaseVersion      = "42.42"
		releaseVersionNewer = "43.43"
	)

	BeforeEach(func() {
		ctx = context.Background()
		logger = log.New(GinkgoWriter, "", 0)

		src1 = new(fakes.ReleaseSource)
		src1.IDReturns("src-1")
		src2 = new(fakes.ReleaseSource)
		src2.IDReturns("src-2")
		src3 = new(fakes.ReleaseSource)
		src3.IDReturns("src-3")
		multiSrc = component.NewReleaseSources(src1, src2, src3)

		requirement = component.Spec{
			Name:            releaseName,
			Version:         releaseVersion,
			StemcellOS:      "not-used",
			StemcellVersion: "not-used",
		}
	})

	Describe("GetMatchedRelease", func() {
		When("one of the release sources has a match", func() {
			var matchedRelease component.Lock

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.ID(),
				}
				src1.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src3.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src2.GetMatchedReleaseReturns(matchedRelease, nil)
			})

			It("returns that match", func() {
				rel, err := multiSrc.GetMatchedRelease(ctx, logger, requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})

		When("none of the release sources has a match", func() {
			BeforeEach(func() {
				src1.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src3.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
				src2.GetMatchedReleaseReturns(component.Lock{}, component.ErrNotFound)
			})
			It("returns no match", func() {
				_, err := multiSrc.GetMatchedRelease(ctx, logger, requirement)
				Expect(err).To(HaveOccurred())
				Expect(component.IsErrNotFound(err)).To(BeTrue())
			})
		})

		When("one of the release sources errors", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("bad stuff happened")
				src1.GetMatchedReleaseReturns(component.Lock{}, expectedErr)
			})

			It("returns that error", func() {
				_, err := multiSrc.GetMatchedRelease(ctx, logger, requirement)
				Expect(err).To(MatchError(ContainSubstring(src1.ID())))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
			})
		})
	})

	Describe("DownloadRelease", func() {
		var (
			releaseID component.Spec
			remote    component.Lock
		)

		BeforeEach(func() {
			releaseID = component.Spec{Name: releaseName, Version: releaseVersion}
			remote = releaseID.Lock().WithRemote(src2.ID(), "/some/remote/path")
		})

		When("the source exists and downloads without error", func() {
			var local component.Local

			BeforeEach(func() {
				l := releaseID.Lock()
				l.SHA1 = "a-sha1"
				local = component.Local{Lock: releaseID.Lock(), LocalPath: "somewhere/on/disk"}
				src2.DownloadReleaseReturns(local, nil)
			})

			It("returns the local release", func() {
				l, err := multiSrc.DownloadRelease(ctx, logger, "somewhere", remote)
				Expect(err).NotTo(HaveOccurred())
				Expect(l).To(Equal(local))

				Expect(src2.DownloadReleaseCallCount()).To(Equal(1))
				_, _, dir, r := src2.DownloadReleaseArgsForCall(0)
				Expect(dir).To(Equal("somewhere"))
				Expect(r).To(Equal(remote))
			})
		})

		When("the source exists and the download errors", func() {
			var expectedErr error
			BeforeEach(func() {
				expectedErr = errors.New("big badda boom")
				src2.DownloadReleaseReturns(component.Local{}, expectedErr)
			})

			It("returns the error", func() {
				_, err := multiSrc.DownloadRelease(ctx, logger, "somewhere", remote)
				Expect(err).To(MatchError(ContainSubstring(src2.ID())))
				Expect(err).To(MatchError(ContainSubstring(expectedErr.Error())))
			})
		})

		When("the source doesn't exist", func() {
			BeforeEach(func() {
				remote.RemoteSource = "no-such-source"
			})

			It("errors", func() {
				_, err := multiSrc.DownloadRelease(ctx, logger, "somewhere", remote)
				Expect(err).To(MatchError(ContainSubstring("couldn't find a release source")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))
				Expect(err).To(MatchError(ContainSubstring(src1.ID())))
				Expect(err).To(MatchError(ContainSubstring(src2.ID())))
				Expect(err).To(MatchError(ContainSubstring(src3.ID())))
			})
		})
	})

	Describe("FindByID", func() {
		When("the source exists", func() {
			It("returns it", func() {
				match, err := multiSrc.FindByID("src-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src1))

				match, err = multiSrc.FindByID("src-2")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src2))

				match, err = multiSrc.FindByID("src-3")
				Expect(err).NotTo(HaveOccurred())
				Expect(match).To(Equal(src3))
			})
		})

		When("the source doesn't exist", func() {
			It("errors", func() {
				_, err := multiSrc.FindByID("no-such-source")
				Expect(err).To(MatchError(ContainSubstring("couldn't find")))
				Expect(err).To(MatchError(ContainSubstring("no-such-source")))

				Expect(err).To(MatchError(ContainSubstring("src-1")))
				Expect(err).To(MatchError(ContainSubstring("src-2")))
				Expect(err).To(MatchError(ContainSubstring("src-3")))
			})
		})
	})

	Describe("FindReleaseVersion", func() {
		When("one of the release sources has a match", func() {
			var matchedRelease component.Lock

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.ID(),
				}
				src1.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
				src2.FindReleaseVersionReturns(matchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns that match", func() {
				rel, err := multiSrc.FindReleaseVersion(ctx, logger, requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
		When("two of the release sources have a match", func() {
			var matchedRelease component.Lock

			BeforeEach(func() {
				unmatchedRelease := component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src1.ID(),
				}
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersionNewer,
					RemotePath:   "/some/path",
					RemoteSource: src2.ID(),
				}
				src1.FindReleaseVersionReturns(unmatchedRelease, nil)
				src2.FindReleaseVersionReturns(matchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns that match", func() {
				rel, err := multiSrc.FindReleaseVersion(ctx, logger, requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
		When("two of the release sources match the same version", func() {
			var matchedRelease component.Lock

			BeforeEach(func() {
				matchedRelease = component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src1.ID(),
				}
				unmatchedRelease := component.Lock{
					Name:         releaseName,
					Version:      releaseVersion,
					RemotePath:   "/some/path",
					RemoteSource: src2.ID(),
				}
				src1.FindReleaseVersionReturns(matchedRelease, nil)
				src2.FindReleaseVersionReturns(unmatchedRelease, nil)
				src3.FindReleaseVersionReturns(component.Lock{}, component.ErrNotFound)
			})

			It("returns the match from the first source", func() {
				rel, err := multiSrc.FindReleaseVersion(ctx, logger, requirement)
				Expect(err).NotTo(HaveOccurred())
				Expect(rel).To(Equal(matchedRelease))
			})
		})
	})

	Describe("YAML", func() {
		When("there are no release sources", func() {
			It("returns an empty List", func() {
				buf := []byte(`banana: []`)
				var reciever struct {
					Sources component.ReleaseSources `yaml:"banana"`
				}
				err := yaml.Unmarshal(buf, &reciever)
				Expect(err).NotTo(HaveOccurred())
				Expect(reciever.Sources.List).To(HaveLen(0))
			})

			It("returns an empty List", func() {
				buf := []byte(`banana:
- type: artifactory
  id: compiled-releases
  artifactory_host: https://artifactory.example.com
  repo: tas
  publishable: true
  username: artifactory_username
  password: artifactory_password
  path_template: path_template
- type: s3
  id: peach
  bucket: some-release-bucket
  path_template: path_template
  region: us-west-1
  access_key_id: $(variable "aws_access_key_id")
  secret_access_key: $(variable "aws_secret_access_key")
  endpoint: s3.example.com
- type: github
  org: cloudfoundry
  id: os-cf
  github_token: $(variable "github_access_token")
- type: bosh.io
  id: public-tarballs
`)
				var receiver struct {
					List component.ReleaseSources `yaml:"banana"`
				}
				err := yaml.Unmarshal(buf, &receiver)
				Expect(err).NotTo(HaveOccurred())
				Expect(receiver.List.List).To(Equal([]component.ReleaseSource{
					&component.ArtifactoryReleaseSource{
						Identifier:      "compiled-releases",
						Publishable:     true,
						ArtifactoryHost: "https://artifactory.example.com",
						Username:        "artifactory_username",
						Password:        "artifactory_password",
						Repo:            "tas",
						PathTemplate:    "path_template",
					},
					&component.S3ReleaseSource{
						Publishable:     false,
						Endpoint:        "s3.example.com",
						Identifier:      "peach",
						Bucket:          "some-release-bucket",
						Region:          "us-west-1",
						AccessKeyId:     "$(variable \"aws_access_key_id\")",
						SecretAccessKey: "$(variable \"aws_secret_access_key\")",
						PathTemplate:    "path_template",
					},
					&component.GitHubReleaseSource{
						Identifier:  "os-cf",
						Publishable: false,
						Org:         "cloudfoundry",
						GithubToken: "$(variable \"github_access_token\")",
					},
					&component.BOSHIOReleaseSource{
						Identifier: "public-tarballs",
						CustomURI:  "",
					},
				}))
			})
		})
	})
})

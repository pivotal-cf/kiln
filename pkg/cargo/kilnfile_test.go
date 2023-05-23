package cargo

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-github/v40/github"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"gopkg.in/yaml.v2"
)

func TestComponentLock_yaml_marshal_order(t *testing.T) {
	const validComponentLockYaml = `name: fake-component-name
sha1: fake-component-sha1
version: fake-version
remote_source: fake-source
remote_path: fake/path/to/fake-component-name
`
	damnit := NewWithT(t)

	cl, err := yaml.Marshal(BOSHReleaseLock{
		Name:         "fake-component-name",
		Version:      "fake-version",
		SHA1:         "fake-component-sha1",
		RemoteSource: "fake-source",
		RemotePath:   "fake/path/to/fake-component-name",
	})

	damnit.Expect(err).NotTo(HaveOccurred())
	damnit.Expect(string(cl)).To(Equal(validComponentLockYaml))
}

func TestKilnfileLock_UpdateReleaseLockWithName(t *testing.T) {
	type args struct {
		name string
		lock BOSHReleaseLock
	}
	tests := []struct {
		name                         string
		KilnfileLock, KilnfileResult KilnfileLock
		args                         args
		wantErr                      bool
	}{
		{name: "empty inputs", wantErr: true},

		{
			name: "lock with name found",
			KilnfileLock: KilnfileLock{
				Releases: []BOSHReleaseLock{
					{Name: "banana"},
				},
			},
			KilnfileResult: KilnfileLock{
				Releases: []BOSHReleaseLock{
					{Name: "orange", Version: "some-version"},
				},
			},
			args: args{
				name: "banana", lock: BOSHReleaseLock{Name: "orange", Version: "some-version"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.KilnfileLock.UpdateReleaseLockWithName(tt.args.name, tt.args.lock); tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.KilnfileResult, tt.KilnfileLock)
		})
	}
}

func TestKilnfile_DownloadBOSHReleaseTarball(t *testing.T) {
	t.Run("bosh.io", func(t *testing.T) {
		var requestURLs []string
		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			requestURLs = append(requestURLs, req.URL.Path)
			res.WriteHeader(http.StatusOK)
			_, _ = res.Write([]byte("just some text"))
		}))
		lock := BOSHReleaseLock{
			Name:         "banana",
			Version:      "1.2.3",
			RemoteSource: ReleaseSourceTypeBOSHIO,
			RemotePath:   server.URL + "/banana-file",
			SHA1:         "bd907aa2280549494055de165f6230d94ce69af1",
		}
		kilnfile := Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{Type: ReleaseSourceTypeBOSHIO},
			},
		}

		dir := t.TempDir()
		ctx := context.Background()
		logger := log.New(io.Discard, "", 0)

		tarballPath, err := kilnfile.DownloadBOSHReleaseTarball(ctx, logger, lock, dir)

		please := NewWithT(t)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(tarballPath).To(BeAnExistingFile())
		please.Expect(requestURLs).To(Equal([]string{
			"/banana-file",
		}))
		buf, err := os.ReadFile(tarballPath)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(string(buf)).To(Equal("just some text"))
	})
	t.Run("artifactory", func(t *testing.T) {
		var requestURLs []string
		server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			requestURLs = append(requestURLs, req.URL.Path)
			res.WriteHeader(http.StatusOK)
			_, _ = res.Write([]byte("just some text"))
		}))
		lock := BOSHReleaseLock{
			Name:         "banana",
			Version:      "1.2.3",
			RemoteSource: ReleaseSourceTypeArtifactory,
			RemotePath:   "/banana-file/tarball.tgz",
			SHA1:         "bd907aa2280549494055de165f6230d94ce69af1",
		}
		kilnfile := Kilnfile{
			ReleaseSources: []ReleaseSourceConfig{
				{
					Type:            ReleaseSourceTypeArtifactory,
					Repo:            "some-repo",
					ArtifactoryHost: server.URL,
					Username:        "cat",
					Password:        "password",
				},
			},
		}

		dir := t.TempDir()
		ctx := context.Background()
		logger := log.New(io.Discard, "", 0)

		tarballPath, err := kilnfile.DownloadBOSHReleaseTarball(ctx, logger, lock, dir)

		please := NewWithT(t)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(tarballPath).To(BeAnExistingFile())
		please.Expect(requestURLs).To(Equal([]string{
			"/artifactory/some-repo/banana-file/tarball.tgz",
		}))
		buf, err := os.ReadFile(tarballPath)
		please.Expect(err).NotTo(HaveOccurred())
		please.Expect(string(buf)).To(Equal("just some text"))
	})
	t.Run("github", func(t *testing.T) {
		t.Run("configure client", func(t *testing.T) {
			logger := log.New(io.Discard, "", 0)
			source := ReleaseSourceConfig{
				Type:        ReleaseSourceTypeGithub,
				GithubToken: "orange",
				Org:         "pivotal",
			}

			clients, err := configureDownloadClient(context.Background(), logger, source)
			please := NewWithT(t)
			please.Expect(err).NotTo(HaveOccurred())
			please.Expect(clients.githubClient).NotTo(BeNil())
		})
		t.Run("download", func(t *testing.T) {
			logger := log.New(io.Discard, "", 0)
			source := ReleaseSourceConfig{
				Type:        ReleaseSourceTypeGithub,
				GithubToken: "orange",
				Org:         "pivotal-cf",
			}
			lock := BOSHReleaseLock{
				Name:         "hello-release",
				Version:      "1.2.3",
				RemoteSource: ReleaseSourceTypeGithub,
				RemotePath:   "https://github.com/pivotal/hello-release/releases/v1.2.3/files/banana-file/tarball.tgz",
				SHA1:         "bd907aa2280549494055de165f6230d94ce69af1",
			}

			dir := t.TempDir()

			server := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				switch req.URL.Path {
				case "/repos/pivotal/hello-release/releases/tags/1.2.3":
					res.WriteHeader(http.StatusNotFound)
				case "/repos/pivotal/hello-release/releases/tags/v1.2.3":
					res.WriteHeader(http.StatusOK)
					buf, _ := json.Marshal(github.RepositoryRelease{
						Assets: []*github.ReleaseAsset{
							{Name: ptr("not-tarball.tgz"), ID: ptr[int64](50)},
							{Name: ptr("hello-release-1.2.3.tgz"), ID: ptr[int64](60)},
						},
					})
					_, _ = res.Write(buf)
				case "/repos/pivotal/hello-release/releases/assets/60":
					res.WriteHeader(http.StatusOK)
					_, _ = res.Write([]byte("just some text"))
				default:
					res.WriteHeader(http.StatusInternalServerError)
					t.Fatalf("unexpected URL path: %s", req.URL.Path)
				}
			}))
			t.Cleanup(server.Close)
			ghClient := github.NewClient(server.Client())
			ghClient.BaseURL, _ = url.Parse(server.URL + "/")

			tarballPath, err := downloadBOSHRelease(context.Background(), logger, source, lock, dir, clients{
				githubClient: ghClient,
			})
			please := NewWithT(t)
			please.Expect(err).NotTo(HaveOccurred())
			please.Expect(tarballPath).To(BeAnExistingFile())
			buf, err := os.ReadFile(tarballPath)
			please.Expect(err).NotTo(HaveOccurred())
			please.Expect(string(buf)).To(Equal("just some text"))
		})
	})
	t.Run("s3", func(t *testing.T) {
		t.Run("releaseSourceByID", func(t *testing.T) {
			_, found := releaseSourceByID(Kilnfile{
				ReleaseSources: []ReleaseSourceConfig{
					{ID: "apple"},
				},
			}, "banana")
			assert.False(t, found)
		})
		t.Run("configureDownloadClient", func(t *testing.T) {
			ctx := context.Background()
			logger := log.New(io.Discard, "", 0)
			_, err := configureDownloadClient(ctx, logger, ReleaseSourceConfig{
				Type: ReleaseSourceTypeS3,
			})
			assert.NoError(t, err)
		})
		t.Run("downloadBOSHReleaseFromS3", func(t *testing.T) {
			ctx := context.Background()
			logger := log.New(io.Discard, "", 0)

			const content = "release.MF"
			var sum = sha1.Sum([]byte(content))
			s3Fake := &fakeS3{
				content: content,
			}

			releasesDirectory := t.TempDir()

			_, err := downloadBOSHRelease(ctx, logger, ReleaseSourceConfig{
				Type: ReleaseSourceTypeS3,
				ID:   "los-angeles",
			}, BOSHReleaseLock{
				Name:         "manny",
				Version:      "1.2.3",
				RemoteSource: "los-angeles",
				RemotePath:   "labrea/mammoth.tgz",
				SHA1:         hex.EncodeToString(sum[:]),
			}, releasesDirectory, clients{
				s3Client: s3Fake,
			})

			assert.NoError(t, err)

			resultContent, err := os.ReadFile(filepath.Join(releasesDirectory, "mammoth.tgz"))
			require.NoError(t, err)
			assert.Equal(t, content, string(resultContent))
		})
	})
}

type fakeS3 struct {
	content string
	s3iface.S3API
}

func (fake fakeS3) GetObjectWithContext(aws.Context, *s3.GetObjectInput, ...request.Option) (*s3.GetObjectOutput, error) {
	return &s3.GetObjectOutput{
		ContentLength: ptr(int64(len(fake.content))),
		Body:          io.NopCloser(bytes.NewReader([]byte(fake.content))),
	}, nil
}

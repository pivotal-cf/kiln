package acceptance_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onsi/gomega/gexec"
	"gopkg.in/src-d/go-billy.v4/osfs"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	test_helpers "github.com/pivotal-cf/kiln/internal/test-helpers"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

var _ = Describe("kiln compile-built-releases", func() {
	const (
		compiledReleasesID = "test-compiled-releases"
		builtReleasesID    = "test-built-releases"
		kilnfileContents   = `
release_sources:
- type: s3
  bucket: ` + compiledReleasesID + `
  region: us-west-1
  access_key_id: my-access-key
  secret_access_key: my-secret-key
  path_template: "{{.Name}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
  publishable: true
  endpoint: %q
- type: s3
  bucket: ` + builtReleasesID + `
  region: us-west-1
  access_key_id: my-access-key
  secret_access_key: my-secret-key
  path_template: "{{.Name}}/{{.Name}}-{{.Version}}.tgz"
  endpoint: %q
`
		kilnfileLockContents = `
releases:
  - name: release-a  # needs to be compiled
    version: 1.2.3
    remote_source: ` + builtReleasesID + `
    remote_path: release-a/release-a-1.2.3.tgz
    sha1: original-sha
  - name: release-b  # already compiled
    version: 42
    remote_source: ` + compiledReleasesID + `
    remote_path: release-b/release-b-42-ubuntu-trusty-22.tgz
    sha1: original-sha
  - name: release-c  # needs to be compiled
    version: 2.3.4
    remote_source: ` + builtReleasesID + `
    remote_path: release-c/release-c-2.3.4.tgz
    sha1: original-sha
stemcell_criteria:
  os: "ubuntu-trusty"
  version: "22"
`
		kilnfileLock2Contents = `
releases:
  - name: release-a  # needs to be compiled
    version: 1.2.3
    remote_source: ` + builtReleasesID + `
    remote_path: release-a/release-a-1.2.3.tgz
    sha1: original-sha
  - name: release-b  # already compiled
    version: 42
    remote_source: ` + compiledReleasesID + `
    remote_path: release-b/release-b-42-ubuntu-trusty-22.tgz
    sha1: original-sha
  - name: release-c  # needs to be compiled
    version: 2.3.4
    remote_source: ` + builtReleasesID + `
    remote_path: release-c/release-c-2.3.4.tgz
    sha1: original-sha
  - name: release-d  # needs to be compiled
    version: 5.6.7
    remote_source: ` + builtReleasesID + `
    remote_path: release-d/release-d-5.6.7.tgz
    sha1: original-sha
stemcell_criteria:
  os: "ubuntu-trusty"
  version: "22"
`
		compiledReleaseAContents = "release-a-compiled-contents"
		compiledReleaseCContents = "release-c-compiled-contents"
		compiledReleaseDContents = "release-d-compiled-contents"
	)

	var (
		tmpDirRoot                                           string
		stemcellTarballPath                                  string
		releasesDirectoryPath                                string
		s3BucketDirectoryPath                                string
		kilnfilePath, kilnfileLockPath                       string
		releaseABuiltSHA, releaseCBuiltSHA, releaseDBuiltSHA string
		releaseAS3LocalPath                                  string
		releaseCS3LocalPath                                  string
		releaseDS3LocalPath                                  string

		deleteDeploymentWasCalled int
		exportReleaseWasCalled    counter
		cleanupWasCalled          int
		s3UploadedFiles           sync.Map
		boshUploadedReleases      sync.Map
		boshUploadedStemcells     sync.Map

		exportReleaseTask int32

		directorInfo []byte

		stemcellSHA string

		serverCert tls.Certificate
		caCert     []byte

		boshServer *httptest.Server
		s3Server   *httptest.Server
	)

	BeforeEach(func() {
		var err error
		tmpDirRoot, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		releasesDirectoryPath = filepath.Join(tmpDirRoot, "releases")
		Expect(
			os.MkdirAll(releasesDirectoryPath, 0700),
		).To(Succeed())

		s3BucketDirectoryPath = filepath.Join(tmpDirRoot, "s3-bucket-releases")
		Expect(
			os.MkdirAll(s3BucketDirectoryPath, 0700),
		).To(Succeed())

		deleteDeploymentWasCalled = 0
		exportReleaseWasCalled.reset()
		cleanupWasCalled = 0

		atomic.StoreInt32(&exportReleaseTask, 1)

		stemcellTarballPath = filepath.Join(tmpDirRoot, "some-stemcell-22.tgz")

		stemcellSHA, err = test_helpers.WriteStemcellTarball(stemcellTarballPath, "ubuntu-trusty", "22", osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		releaseAS3LocalPath = filepath.Join(s3BucketDirectoryPath, "release-a.tgz")
		releaseABuiltSHA, err = test_helpers.WriteReleaseTarball(releaseAS3LocalPath, "release-a", "1.2.3", osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		releaseCS3LocalPath = filepath.Join(s3BucketDirectoryPath, "release-c.tgz")
		releaseCBuiltSHA, err = test_helpers.WriteReleaseTarball(releaseCS3LocalPath, "release-c", "2.3.4", osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		releaseDS3LocalPath = filepath.Join(s3BucketDirectoryPath, "release-d.tgz")
		releaseDBuiltSHA, err = test_helpers.WriteReleaseTarball(releaseDS3LocalPath, "release-d", "5.6.7", osfs.New(""))
		Expect(err).NotTo(HaveOccurred())

		caCert, serverCert, err = GenerateCertificateChain()
		Expect(err).NotTo(HaveOccurred())

		boshUploadedReleases = sync.Map{}

		boshServer = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			matched, err := regexp.MatchString("/deployments/compile-built-releases.*", req.URL.Path)
			Expect(err).NotTo(HaveOccurred())

			if matched {
				if req.Method == "DELETE" {
					deleteDeploymentWasCalled++
					w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
					w.WriteHeader(http.StatusFound)
					return
				}

				w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
				w.WriteHeader(http.StatusFound)
			}

			switch req.URL.Path {
			case "/oauth/token":
				w.Write([]byte(`{
					"access_token": "some-weird-token",
					"token_type": "Bearer",
					"expires_in": 3600
				}`))
			case "/info":
				w.Write(directorInfo)
			case "/deployments":
				if req.Method == "POST" {
					w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
					w.WriteHeader(http.StatusFound)
				}
			case "/stemcells":
				w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
				w.WriteHeader(http.StatusFound)

				manifestBody, sha1 := readManifestFromTarball(req.Body, "stemcell.MF")

				var manifest struct {
					OperatingSystem string `yaml:"operating_system"`
					Version         string `yaml:"version"`
				}

				err = yaml.Unmarshal(manifestBody, &manifest)
				Expect(err).NotTo(HaveOccurred())

				boshUploadedStemcells.Store(fmt.Sprintf("%s-%s", manifest.OperatingSystem, manifest.Version), sha1)
			case "/releases":
				w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
				w.WriteHeader(http.StatusFound)

				manifestBody, sha1 := readManifestFromTarball(req.Body, "release.MF")

				var manifest struct {
					Name    string `yaml:"name"`
					Version string `yaml:"version"`
				}

				err = yaml.Unmarshal(manifestBody, &manifest)
				Expect(err).NotTo(HaveOccurred())

				boshUploadedReleases.Store(fmt.Sprintf("%s-%s", manifest.Name, manifest.Version), sha1)
			case "/releases/export":
				exportReleaseWasCalled.increment()

				var parsedBody struct {
					ReleaseVersion string `yaml:"release_name"`
				}
				Expect(
					yaml.NewDecoder(req.Body).Decode(&parsedBody),
				).To(Succeed())

				var desiredTaskID int
				switch parsedBody.ReleaseVersion {
				case "release-a":
					desiredTaskID = 10
				case "release-c":
					desiredTaskID = 20
				case "release-d":
					desiredTaskID = 30
				}

				w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/%d", req.Host, desiredTaskID))
				w.WriteHeader(http.StatusFound)
			case "/tasks/1":
				w.Write([]byte(`{"id":1, "state": "done"}`))
			case "/tasks/1/output":
				w.Write([]byte(`{"blobstore_id": "12345", "sha1": "0732aaa8a43e0776e549f5036ce2aff2ae735572"}`))

			case "/tasks/10":
				w.Write([]byte(`{"id":10, "state": "done"}`))
			case "/tasks/10/output":
				s := sha1.New()
				io.Copy(s, strings.NewReader(compiledReleaseAContents))
				sha1 := hex.EncodeToString(s.Sum(nil))
				w.Write([]byte(
					fmt.Sprintf(`{"blobstore_id": %q, "sha1": %q}`, "34567", sha1),
				))
			case "/resources/34567":
				w.Write([]byte(compiledReleaseAContents))

			case "/tasks/20":
				w.Write([]byte(`{"id":20, "state": "done"}`))
			case "/tasks/20/output":
				s := sha1.New()
				io.Copy(s, strings.NewReader(compiledReleaseCContents))
				sha1 := hex.EncodeToString(s.Sum(nil))
				w.Write([]byte(
					fmt.Sprintf(`{"blobstore_id": %q, "sha1": %q}`, "67890", sha1),
				))
			case "/resources/67890":
				w.Write([]byte(compiledReleaseCContents))

			case "/tasks/30":
				w.Write([]byte(`{"id":30, "state": "done"}`))
			case "/tasks/30/output":
				s := sha1.New()
				io.Copy(s, strings.NewReader(compiledReleaseDContents))
				sha1 := hex.EncodeToString(s.Sum(nil))
				w.Write([]byte(
					fmt.Sprintf(`{"blobstore_id": %q, "sha1": %q}`, "45678", sha1),
				))
			case "/resources/45678":
				w.Write([]byte(compiledReleaseDContents))

			case "/cleanup":
				cleanupWasCalled++
				w.Header().Set("Location", fmt.Sprintf("https://%s/tasks/1", req.Host))
				w.WriteHeader(http.StatusFound)
			default:
				panic(fmt.Sprintf("Fake BOSH director received a request with unhandled path: %#v", req))
			}
		}))

		boshServer.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert}}
		directorInfo = []byte(fmt.Sprintf(`{
							"user":"some-user",
							"user_authentication": {
								"type":"uaa",
								"options": {
									"url":"https://%s"
								}
							}
						}`, boshServer.Listener.Addr().String()))

		boshServer.StartTLS()

		s3UploadedFiles = sync.Map{}

		s3Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			switch req.Method {
			case "GET":
				var filePath string

				switch req.URL.Path {
				case fmt.Sprintf("/%s/%s/%s-%s.tgz", builtReleasesID, "release-a", "release-a", "1.2.3"):
					filePath = releaseAS3LocalPath
				case fmt.Sprintf("/%s/%s/%s-%s.tgz", builtReleasesID, "release-c", "release-c", "2.3.4"):
					filePath = releaseCS3LocalPath
				case fmt.Sprintf("/%s/%s/%s-%s.tgz", builtReleasesID, "release-d", "release-d", "5.6.7"):
					filePath = releaseDS3LocalPath
				}

				byteRange := strings.TrimPrefix(req.Header.Get("RANGE"), "bytes=")
				firstAndLastByte := strings.Split(byteRange, "-")
				firstByte, err := strconv.ParseInt(firstAndLastByte[0], 10, 64)
				Expect(err).NotTo(HaveOccurred())

				lastByte, err := strconv.ParseInt(firstAndLastByte[1], 10, 64)
				Expect(err).NotTo(HaveOccurred())

				f, err := os.Open(filePath)
				Expect(err).NotTo(HaveOccurred())

				stat, err := f.Stat()
				Expect(err).NotTo(HaveOccurred())

				size := stat.Size()

				if firstByte > size {
					w.WriteHeader(416)
					return
				}

				fileSection := io.NewSectionReader(f, firstByte, lastByte-firstByte)
				_, err = io.Copy(w, fileSection)
				Expect(err).ToNot(HaveOccurred())

				return
			case "PUT":
				contents, err := ioutil.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())

				s3UploadedFiles.Store(req.URL.Path, string(contents))
			case "HEAD":
				w.WriteHeader(404)
				return

			default:
				panic(fmt.Sprintf("Fake S3 server received a request with unhandled path: %#v", req))
			}
		}))
		s3Server.Start()

		kilnfilePath = filepath.Join(tmpDirRoot, "Kilnfile")
		kilnfileLockPath = kilnfilePath + ".lock"

		Expect(
			ioutil.WriteFile(kilnfilePath, []byte(fmt.Sprintf(kilnfileContents, s3Server.URL, s3Server.URL)), 0600),
		).To(Succeed())
		Expect(
			ioutil.WriteFile(kilnfileLockPath, []byte(kilnfileLockContents), 0600),
		).To(Succeed())
	})

	AfterEach(func() {
		boshServer.Close()
		Expect(
			os.RemoveAll(tmpDirRoot),
		).To(Succeed())
	})

	It("compiles a release for a given stemcell", func() {
		command := exec.Command(pathToMain, "compile-built-releases",
			"--kilnfile", kilnfilePath,
			"--releases-directory", releasesDirectoryPath,
			"--stemcell-file", stemcellTarballPath,
			"--upload-target-id", compiledReleasesID,
		)

		command.Env = append(command.Env, fmt.Sprintf("BOSH_ENVIRONMENT=%s", boshServer.URL))
		command.Env = append(command.Env, "BOSH_CLIENT=some-bosh-user")
		command.Env = append(command.Env, "BOSH_CLIENT_SECRET=some-bosh-password")
		command.Env = append(command.Env, fmt.Sprintf("BOSH_CA_CERT=%s", string(caCert)))

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session.Wait(2 * time.Second)).Should(gexec.Exit(0))

		// download releases to releases directory
		built1File, err := os.Open(filepath.Join(releasesDirectoryPath, "release-a-1.2.3.tgz"))
		Expect(err).NotTo(HaveOccurred())
		defer built1File.Close()

		s := sha1.New()
		io.Copy(s, built1File)
		actualSHA1 := hex.EncodeToString(s.Sum(nil))

		expectedStat, _ := os.Stat(releaseAS3LocalPath)
		actualStat, _ := os.Stat(filepath.Join(releasesDirectoryPath, "release-a-1.2.3.tgz"))
		Expect(actualSHA1).To(Equal(releaseABuiltSHA), fmt.Sprintf("expected = %#v\nactual = %#v\n", expectedStat, actualStat))

		built2File, err := os.Open(filepath.Join(releasesDirectoryPath, "release-c-2.3.4.tgz"))
		Expect(err).NotTo(HaveOccurred())
		defer built2File.Close()

		s = sha1.New()
		io.Copy(s, built2File)
		actualSHA1 = hex.EncodeToString(s.Sum(nil))
		Expect(actualSHA1).To(Equal(releaseCBuiltSHA), fmt.Sprintf("expected = %#v\nactual = %#v\n", expectedStat, actualStat))

		// upload releases to director
		actualReleaseSHA1, ok := boshUploadedReleases.Load("release-a-1.2.3")
		Expect(ok).To(BeTrue())
		Expect(actualReleaseSHA1.(string)).To(Equal(releaseABuiltSHA))

		actualReleaseSHA1, ok = boshUploadedReleases.Load("release-c-2.3.4")
		Expect(ok).To(BeTrue())
		Expect(actualReleaseSHA1.(string)).To(Equal(releaseCBuiltSHA))

		// upload stemcell to director
		actualStemcellSHA1, ok := boshUploadedStemcells.Load("ubuntu-trusty-22")
		Expect(ok).To(BeTrue())
		Expect(actualStemcellSHA1.(string)).To(Equal(stemcellSHA))

		// export releases
		Expect(exportReleaseWasCalled.count()).To(Equal(2))

		compiledReleaseContents, err := ioutil.ReadFile(filepath.Join(releasesDirectoryPath, "release-a-1.2.3-ubuntu-trusty-22.tgz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(compiledReleaseContents).To(Equal([]byte(compiledReleaseAContents)))

		compiledReleaseContents, err = ioutil.ReadFile(filepath.Join(releasesDirectoryPath, "release-c-2.3.4-ubuntu-trusty-22.tgz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(compiledReleaseContents).To(Equal([]byte(compiledReleaseCContents)))

		// upload releases to compiled bucket
		uploadedContents, ok := s3UploadedFiles.Load(fmt.Sprintf("/%s/release-a/release-a-1.2.3-ubuntu-trusty-22.tgz", compiledReleasesID))
		Expect(ok).To(BeTrue())
		Expect(uploadedContents.(string)).To(Equal(compiledReleaseAContents))

		uploadedContents, ok = s3UploadedFiles.Load(fmt.Sprintf("/%s/release-c/release-c-2.3.4-ubuntu-trusty-22.tgz", compiledReleasesID))
		Expect(ok).To(BeTrue())
		Expect(uploadedContents.(string)).To(Equal(compiledReleaseCContents))

		// delete deployment
		Expect(deleteDeploymentWasCalled).To(Equal(1))

		// clean up releases
		Expect(cleanupWasCalled).To(Equal(1))
	})

	When("using parallel option", func() {
		BeforeEach(func() {
			Expect(
				ioutil.WriteFile(kilnfileLockPath, []byte(kilnfileLock2Contents), 0600),
			).To(Succeed())
		})

		It("compiles releases for a given stemcell", func() {
			command := exec.Command(pathToMain, "compile-built-releases",
				"--kilnfile", kilnfilePath,
				"--releases-directory", releasesDirectoryPath,
				"--stemcell-file", stemcellTarballPath,
				"--upload-target-id", compiledReleasesID,
				"--parallel", "2",
			)

			command.Env = append(command.Env, fmt.Sprintf("BOSH_ENVIRONMENT=%s", boshServer.URL))
			command.Env = append(command.Env, "BOSH_CLIENT=some-bosh-user")
			command.Env = append(command.Env, "BOSH_CLIENT_SECRET=some-bosh-password")
			command.Env = append(command.Env, fmt.Sprintf("BOSH_CA_CERT=%s", string(caCert)))

			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session.Wait(2 * time.Second)).Should(gexec.Exit(0))

			// download releases to releases directory
			built1File, err := os.Open(filepath.Join(releasesDirectoryPath, "release-a-1.2.3.tgz"))
			Expect(err).NotTo(HaveOccurred())
			defer built1File.Close()

			s := sha1.New()
			io.Copy(s, built1File)
			actualSHA1 := hex.EncodeToString(s.Sum(nil))

			expectedStat, _ := os.Stat(releaseAS3LocalPath)
			actualStat, _ := os.Stat(filepath.Join(releasesDirectoryPath, "release-a-1.2.3.tgz"))
			Expect(actualSHA1).To(Equal(releaseABuiltSHA), fmt.Sprintf("expected = %#v\nactual = %#v\n", expectedStat, actualStat))

			built2File, err := os.Open(filepath.Join(releasesDirectoryPath, "release-c-2.3.4.tgz"))
			Expect(err).NotTo(HaveOccurred())
			defer built2File.Close()

			s = sha1.New()
			io.Copy(s, built2File)
			actualSHA1 = hex.EncodeToString(s.Sum(nil))
			Expect(actualSHA1).To(Equal(releaseCBuiltSHA), fmt.Sprintf("expected = %#v\nactual = %#v\n", expectedStat, actualStat))

			built3File, err := os.Open(filepath.Join(releasesDirectoryPath, "release-d-5.6.7.tgz"))
			Expect(err).NotTo(HaveOccurred())
			defer built3File.Close()

			s = sha1.New()
			io.Copy(s, built3File)
			actualSHA1 = hex.EncodeToString(s.Sum(nil))
			Expect(actualSHA1).To(Equal(releaseDBuiltSHA), fmt.Sprintf("expected = %#v\nactual = %#v\n", expectedStat, actualStat))

			// upload releases to director
			actualReleaseSHA1, ok := boshUploadedReleases.Load("release-a-1.2.3")
			Expect(ok).To(BeTrue())
			Expect(actualReleaseSHA1.(string)).To(Equal(releaseABuiltSHA))

			actualReleaseSHA1, ok = boshUploadedReleases.Load("release-c-2.3.4")
			Expect(ok).To(BeTrue())
			Expect(actualReleaseSHA1.(string)).To(Equal(releaseCBuiltSHA))

			actualReleaseSHA1, ok = boshUploadedReleases.Load("release-d-5.6.7")
			Expect(ok).To(BeTrue())
			Expect(actualReleaseSHA1.(string)).To(Equal(releaseDBuiltSHA))

			// upload stemcell to director
			actualStemcellSHA1, ok := boshUploadedStemcells.Load("ubuntu-trusty-22")
			Expect(ok).To(BeTrue())
			Expect(actualStemcellSHA1.(string)).To(Equal(stemcellSHA))

			// export releases
			Expect(exportReleaseWasCalled.count()).To(Equal(3))

			compiledReleaseContents, err := ioutil.ReadFile(filepath.Join(releasesDirectoryPath, "release-a-1.2.3-ubuntu-trusty-22.tgz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(compiledReleaseContents).To(Equal([]byte(compiledReleaseAContents)))

			compiledReleaseContents, err = ioutil.ReadFile(filepath.Join(releasesDirectoryPath, "release-c-2.3.4-ubuntu-trusty-22.tgz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(compiledReleaseContents).To(Equal([]byte(compiledReleaseCContents)))

			compiledReleaseContents, err = ioutil.ReadFile(filepath.Join(releasesDirectoryPath, "release-d-5.6.7-ubuntu-trusty-22.tgz"))
			Expect(err).NotTo(HaveOccurred())
			Expect(compiledReleaseContents).To(Equal([]byte(compiledReleaseDContents)))

			// upload releases to compiled bucket
			uploadedContents, ok := s3UploadedFiles.Load(fmt.Sprintf("/%s/release-a/release-a-1.2.3-ubuntu-trusty-22.tgz", compiledReleasesID))
			Expect(ok).To(BeTrue())
			Expect(uploadedContents.(string)).To(Equal(compiledReleaseAContents))

			uploadedContents, ok = s3UploadedFiles.Load(fmt.Sprintf("/%s/release-c/release-c-2.3.4-ubuntu-trusty-22.tgz", compiledReleasesID))
			Expect(ok).To(BeTrue())
			Expect(uploadedContents.(string)).To(Equal(compiledReleaseCContents))

			uploadedContents, ok = s3UploadedFiles.Load(fmt.Sprintf("/%s/release-d/release-d-5.6.7-ubuntu-trusty-22.tgz", compiledReleasesID))
			Expect(ok).To(BeTrue())
			Expect(uploadedContents.(string)).To(Equal(compiledReleaseDContents))

			// delete deployment
			Expect(deleteDeploymentWasCalled).To(Equal(2))

			// clean up releases
			Expect(cleanupWasCalled).To(Equal(1))
		})
	})

	It("updates the Kilnfile.lock with the compiled releases", func() {
		command := exec.Command(pathToMain, "compile-built-releases",
			"--kilnfile", kilnfilePath,
			"--releases-directory", releasesDirectoryPath,
			"--stemcell-file", stemcellTarballPath,
			"--upload-target-id", compiledReleasesID,
		)

		command.Env = append(command.Env, fmt.Sprintf("BOSH_ENVIRONMENT=%s", boshServer.URL))
		command.Env = append(command.Env, "BOSH_CLIENT=some-bosh-user")
		command.Env = append(command.Env, "BOSH_CLIENT_SECRET=some-bosh-password")
		command.Env = append(command.Env, fmt.Sprintf("BOSH_CA_CERT=%s", string(caCert)))

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(session.Wait(2 * time.Second)).Should(gexec.Exit(0))

		file, err := os.Open(kilnfilePath + ".lock")
		Expect(err).NotTo(HaveOccurred())

		var updatedLockfile cargo.KilnfileLock
		Expect(
			yaml.NewDecoder(file).Decode(&updatedLockfile),
		).To(Succeed())

		s := sha1.New()
		io.Copy(s, strings.NewReader(compiledReleaseAContents))
		releaseASha1 := hex.EncodeToString(s.Sum(nil))

		s = sha1.New()
		io.Copy(s, strings.NewReader(compiledReleaseCContents))
		releaseCSha1 := hex.EncodeToString(s.Sum(nil))

		Expect(updatedLockfile).To(Equal(cargo.KilnfileLock{
			Releases: []cargo.ReleaseLock{
				{
					Name:         "release-a",
					Version:      "1.2.3",
					RemoteSource: compiledReleasesID,
					RemotePath:   "release-a/release-a-1.2.3-ubuntu-trusty-22.tgz",
					SHA1:         releaseASha1,
				},
				{
					Name:         "release-b",
					Version:      "42",
					RemoteSource: compiledReleasesID,
					RemotePath:   "release-b/release-b-42-ubuntu-trusty-22.tgz",
					SHA1:         "original-sha",
				},
				{
					Name:         "release-c",
					Version:      "2.3.4",
					RemoteSource: compiledReleasesID,
					RemotePath:   "release-c/release-c-2.3.4-ubuntu-trusty-22.tgz",
					SHA1:         releaseCSha1,
				},
			},
			Stemcell: cargo.Stemcell{OS: "ubuntu-trusty", Version: "22"},
		}))
	})
})

func readManifestFromTarball(body io.Reader, manifestPath string) ([]byte, string) {
	bodyBits, err := ioutil.ReadAll(body)
	Expect(err).NotTo(HaveOccurred())

	buf := bytes.NewBuffer(bodyBits)
	s := sha1.New()
	_, err = io.Copy(s, buf)
	Expect(err).NotTo(HaveOccurred())

	sha1 := hex.EncodeToString(s.Sum(nil))
	reader := bytes.NewReader(bodyBits)
	zipReader, err := gzip.NewReader(reader)
	Expect(err).NotTo(HaveOccurred())

	tarReader := tar.NewReader(zipReader)
	Expect(err).NotTo(HaveOccurred())

	var manifestBody []byte
	for header, err := tarReader.Next(); ; {
		if err != nil {
			Expect(err).To(Equal(io.EOF))
		}
		if header.Name == manifestPath {
			manifestBody, err = ioutil.ReadAll(tarReader)
			Expect(err).NotTo(HaveOccurred())
			break
		}
	}
	return manifestBody, sha1
}

func GenerateCertificateChain() ([]byte, tls.Certificate, error) {
	// create a template for the CA certificate
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1234),
		Subject: pkix.Name{
			CommonName:   "ca.localhost",
			Country:      []string{"US"},
			Organization: []string{"Pivotal"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(5, 5, 5),
		SubjectKeyId:          []byte{1, 2, 3, 4, 5},
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	// generate the CA private key used to sign certificates
	caPrivatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// create a self-signed certificate for the CA. template = parent
	caCert, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivatekey.PublicKey, caPrivatekey)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// read the signed CA certificate back into the template
	caTemplate, err = x509.ParseCertificate(caCert)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// verify that the CA certificate is validly self-signed
	err = caTemplate.CheckSignatureFrom(caTemplate)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// encode the CA certificate as PEM bytes
	caCert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert,
	})

	// create a template for the server certificate
	template := &x509.Certificate{
		SerialNumber: big.NewInt(7890),
		Subject: pkix.Name{
			CommonName:   "server.localhost",
			Country:      []string{"US"},
			Organization: []string{"Pivotal"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(5, 5, 5),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	// generate the private key used in the TLS key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// encode the privateKey to PEM bytes
	key := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// create a certificate that is signed by the CA. the CA is the parent, and the CA private key is used to sign
	serverCert, err := x509.CreateCertificate(rand.Reader, template, caTemplate, &privateKey.PublicKey, caPrivatekey)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// read the signed certificate back into the template
	template, err = x509.ParseCertificate(serverCert)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// verify the the certificate is signed by the CA
	err = template.CheckSignatureFrom(caTemplate)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	// encode the certificate to PEM bytes
	serverCert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCert,
	})

	// generate a key pair with the server certificate and private key
	keyPair, err := tls.X509KeyPair(serverCert, key)
	if err != nil {
		return nil, tls.Certificate{}, err
	}

	return caCert, keyPair, nil
}

type counter struct {
	n   int
	mut sync.Mutex
}

func (c *counter) reset() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.n = 0
}

func (c *counter) increment() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.n++
}

func (c *counter) count() int {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.n
}

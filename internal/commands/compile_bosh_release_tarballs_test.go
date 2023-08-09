package commands_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/pkg/cargo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-cli/v7/director"
	"github.com/pivotal-cf/kiln/pkg/directorclient"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
)

var _ = Describe("compile-bosh-release-tarballs", func() {
	var (
		cmd              commands.CompileBOSHReleaseTarballs
		fakeCompileFunc  *fakes.CompileBOSHReleaseTarballsFunc
		logOutput        bytes.Buffer
		newBOSHDirector  func(configuration directorclient.Configuration) (director.Director, error)
		fakeBOSHDirector *fakes.BOSHDirector
	)
	BeforeEach(func() {
		cmd = commands.CompileBOSHReleaseTarballs{}
		fakeCompileFunc = new(fakes.CompileBOSHReleaseTarballsFunc)
		fakeBOSHDirector = new(fakes.BOSHDirector)
		newBOSHDirector = func(directorclient.Configuration) (director.Director, error) {
			return fakeBOSHDirector, nil
		}
	})
	AfterEach(func() {
		logOutput.Reset()

		fakeCompileFunc = nil
		fakeBOSHDirector = nil
		newBOSHDirector = nil
	})
	JustBeforeEach(func() {
		cmd.CompileBOSHReleaseTarballsFunc = fakeCompileFunc.Spy
		cmd.Logger = log.New(&logOutput, "", 0)
		cmd.NewDirectorFunc = newBOSHDirector
	})

	When("a BOSH release tarball with compiled tarballs", func() {
		It("passes the releases in the releases directory to the compile func", func() {
			dir, err := os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			rd := filepath.Join(dir, "releases")
			err = os.MkdirAll(rd, 0o700)
			Expect(err).NotTo(HaveOccurred())
			testdataCompiledReleaseTarballPath := filepath.Join("testdata", "compile_bosh_release_tarballs", "bpm-1.1.21-ubuntu-xenial-621.463.tgz")
			err = copyFile(filepath.Join(rd, filepath.Base(testdataCompiledReleaseTarballPath)), testdataCompiledReleaseTarballPath)
			Expect(err).NotTo(HaveOccurred())
			kf := filepath.Join(dir, "Kilnfile")
			err = os.WriteFile(kf+".lock", []byte(`{"stemcell_criteria": {"os": "peach", "version": "4.5"}}`), 0o600)
			Expect(err).NotTo(HaveOccurred())

			fakeBOSHDirector.InfoReturns(director.Info{}, nil)

			err = cmd.Execute([]string{"--releases-directory=" + rd, "--kilnfile=" + kf})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCompileFunc.CallCount()).To(Equal(1))
			Expect(fakeCompileFunc.CallCount()).To(Equal(1))

			_, _, _, _, _, toCompile := fakeCompileFunc.ArgsForCall(0)
			Expect(toCompile).To(HaveLen(0))
		})
	})

	When("a BOSH release tarball without compiled tarballs", func() {
		It("passes the releases in the releases directory to the compile func", func() {
			dir, err := os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())

			rd := filepath.Join(dir, "releases")
			err = os.MkdirAll(rd, 0o700)
			Expect(err).NotTo(HaveOccurred())
			testdataReleasePath := filepath.Join("testdata", "compile_bosh_release_tarballs", "bpm-1.1.21.tgz")
			err = copyFile(filepath.Join(rd, filepath.Base("bpm-1.1.21.tgz")), testdataReleasePath)
			Expect(err).NotTo(HaveOccurred())
			kf := filepath.Join(dir, "Kilnfile")
			// language=yaml
			lockContents := `{"stemcell_criteria": {"os": "peach", "version": "4.5"}, "releases": [{"name": "bpm", "version": "1.1.21"}]}`
			err = os.WriteFile(kf+".lock", []byte(lockContents), 0o600)
			Expect(err).NotTo(HaveOccurred())

			fakeBOSHDirector.InfoReturns(director.Info{}, nil)
			fakeCompileFunc.Stub = func(ctx context.Context, logger *log.Logger, d director.Director, stemcell cargo.Stemcell, i int, tarball ...cargo.BOSHReleaseTarball) ([]cargo.BOSHReleaseTarball, error) {
				compiledTarballPath := filepath.Join("testdata", "compile_bosh_release_tarballs", "bpm-1.1.21-ubuntu-xenial-621.463.tgz")
				err := copyFile(filepath.Join(rd, filepath.Base(compiledTarballPath)), compiledTarballPath)
				if err != nil {
					panic(err)
				}
				t, err := cargo.ReadBOSHReleaseTarball(compiledTarballPath)
				if err != nil {
					panic(err)
				}
				return []cargo.BOSHReleaseTarball{t}, nil
			}

			err = cmd.Execute([]string{"--releases-directory=" + rd, "--kilnfile=" + kf})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeCompileFunc.CallCount()).To(Equal(1))
			Expect(fakeCompileFunc.CallCount()).To(Equal(1))

			_, _, _, _, _, toCompile := fakeCompileFunc.ArgsForCall(0)
			Expect(toCompile).To(HaveLen(1))

			lock, err := cargo.ReadKilnfileLock(kf)
			Expect(err).NotTo(HaveOccurred())
			Expect(lock.Releases).To(Equal([]cargo.BOSHReleaseTarballLock{
				{
					Name:    "bpm",
					Version: "1.1.21",
					// note the sha and release source/path have not been modified
				},
			}))
		})
	})

	Describe("NewCompileBOSHReleaseTarballs", func() {
		It("sets required fields", func() {
			cmd := commands.NewCompileBOSHReleaseTarballs()
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.CompileBOSHReleaseTarballsFunc).NotTo(BeNil())
			Expect(cmd.Logger).NotTo(BeNil())
		})
	})

	Describe("Usage", func() {
		When("Printed", func() {
			It("prints helpful output", func() {
				usage := cmd.Usage()
				out, err := jhanda.PrintUsage(usage.Flags)
				Expect(err).NotTo(HaveOccurred())

				Expect(out).To(ContainSubstring("BOSH_ENVIRONMENT"))
				Expect(out).To(ContainSubstring("BOSH_ENV_NAME"))
				Expect(out).To(ContainSubstring("BOSH_ALL_PROXY"))
				Expect(out).To(ContainSubstring("BOSH_CLIENT"))
				Expect(out).To(ContainSubstring("BOSH_CLIENT_SECRET"))
				Expect(out).To(ContainSubstring("BOSH_CA_CERT"))

				Expect(out).To(ContainSubstring("kilnfile"))
				Expect(out).To(ContainSubstring("releases-directory"))
			})
		})

		When("Long Help", func() {
			It("prints helpful output", func() {
				usage := cmd.Usage()
				Expect(usage.Description).NotTo(BeEmpty())
			})
		})

		When("Short Help", func() {
			It("prints helpful output", func() {
				usage := cmd.Usage()
				Expect(usage.ShortDescription).NotTo(BeEmpty())
			})
		})
	})
})

func copyFile(dst, src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(srcFile)
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(dstFile)
	_, err = io.Copy(dstFile, srcFile)
	return err
}

/*
{
						FilePath: "bpm-1.1.21-ubuntu-xenial-621.463.tgz",
						SHA1:     "d260e4a628087f030dbc4d66bd6f688ec979b5bb",
						Manifest: cargo.BOSHReleaseManifest{
							Name:               "bpm",
							Version:            "1.1.21",
							CommitHash:         "fd88358",
							UncommittedChanges: false,
							CompiledPackages: []cargo.CompiledBOSHReleasePackage{
								{
									Name:         "bpm",
									Version:      "be375c78c703cea04667ea7cbbc6d024bb391182",
									Fingerprint:  "be375c78c703cea04667ea7cbbc6d024bb391182",
									SHA1:         "b67ab0ceb0cab69a170521bb1a77c91a8546ac21",
									Dependencies: []string{"golang-1-linux", "bpm-runc", "tini"},
									Stemcell:     "ubuntu-xenial/621.463",
								},
								{
									Name:         "test-server",
									Version:      "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
									Fingerprint:  "12eba471a2c3dddb8547ef03c23a3231d1f62e6c",
									SHA1:         "7ab0c2066c63eb5c5dd2c06d35b73376f4ad9a81",
									Dependencies: []string{"golang-1-linux"},
									Stemcell:     "ubuntu-xenial/621.463",
								},
								{
									Name:         "bpm-runc",
									Version:      "464c6e6611f814bd12016156bf3e682486f34672",
									Fingerprint:  "464c6e6611f814bd12016156bf3e682486f34672",
									SHA1:         "bacd602ee0830a30c17b7a502aa0021a6739a3ff",
									Dependencies: []string{"golang-1-linux"},
									Stemcell:     "ubuntu-xenial/621.463",
								},
								{
									Name:         "golang-1-linux",
									Version:      "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
									Fingerprint:  "2336380dbf01a44020813425f92be34685ce118bf4767406c461771cfef14fc9",
									SHA1:         "e8dad3e51eeb5f5fb41dd56bbb8a3ec9655bd4f7",
									Dependencies: []string{},
									Stemcell:     "ubuntu-xenial/621.463",
								},
								{
									Name:         "tini",
									Version:      "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
									Fingerprint:  "3d7b02f3eeb480b9581bec4a0096dab9ebdfa4bc",
									SHA1:         "347c76d509ad4b82d99bbbb4b291768ff90b0fba",
									Dependencies: []string{},
									Stemcell:     "ubuntu-xenial/621.463",
								},
							},
							Packages: []cargo.BOSHReleasePackage(nil)},
					},
*/

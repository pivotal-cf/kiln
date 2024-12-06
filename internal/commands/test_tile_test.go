package commands_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/test"
)

func init() {
	format.MaxLength = 100000
}

// counterfeiter does not handle publicly exported type function spy generation super well.
// So I am telling it to generate the spy off of a private type alias. This works but is a bit confusing.
//

// nolint:unused
//
//goland:noinspection GoUnusedType
// type tileTestFunction = commands.TileTestFunction

var _ = Describe("kiln test", func() {
	var output bytes.Buffer

	AfterEach(func() {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		vendorDir := filepath.Join(filepath.Dir(filepath.Dir(wd)), "vendor")

		info, err := os.Stat(vendorDir)
		if err == nil && info.IsDir() { // no error
			_ = os.RemoveAll(vendorDir)
		}

		output.Reset()
	})

	When("no test arguments are passed", func() {
		It("runs all the tests with initialized collaborators", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))
			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.AbsoluteTileDirectory).To(BeADirectory())
			Expect(configuration.RunAll).To(BeTrue())
			Expect(output.String()).To(ContainSubstring("hello"))
		})
	})

	When("the tile directory does not exist", func() {
		It("returns an error", func() {
			dir, err := os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			tilePath := filepath.Join(dir, "some-dir")

			args := []string{
				"--git-auth-token", "some-auth-token",
				"--tile-path", tilePath,
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err = commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).To(MatchError(ContainSubstring("failed to get information about --tile-path")))
		})
	})

	When("the verbose flag argument is passed or the silent flag argument is not passed", func() {
		It("runs all the tests with initalized collaborators", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--verbose",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, _ := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(output.String()).To(ContainSubstring("hello"))
		})
	})

	When("the silent flag argument is passed", func() {
		It("runs all the tests without initalized collaborators", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--silent",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, _ := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(output.String()).To(BeEmpty())
		})
	})

	When("the manifest test is enabled", func() {
		It("sets the manifest configuration flag", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--manifest",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.RunManifest).To(BeTrue())
			Expect(configuration.RunMetadata).To(BeFalse())
			Expect(configuration.RunMigrations).To(BeFalse())
		})
	})

	When("the migrations test is enabled", func() {
		It("sets the migrations configuration flag", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--migrations",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.RunManifest).To(BeFalse())
			Expect(configuration.RunMetadata).To(BeFalse())
			Expect(configuration.RunMigrations).To(BeTrue())
		})
	})

	When("the stability test is enabled", func() {
		It("sets the metadata configuration flag", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--stability",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.RunManifest).To(BeFalse())
			Expect(configuration.RunMetadata).To(BeTrue())
			Expect(configuration.RunMigrations).To(BeFalse())
		})
	})

	When("ginkgo flag arguments are passed", func() {
		It("sets the metadata configuration flag", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
				"--ginkgo-flags=peach pair",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.GinkgoFlags).To(Equal("peach pair"))
		})
	})

	When("environment variables flags arguments are passed", func() {
		When("the using the short environment variable flag", func() {
			It("sets the metadata configuration flag", func() {
				args := []string{
					"--git-auth-token", "some-auth-token",
					"-e=PEAR=on-pizza",
				}

				fakeTestFunc := fakes.TestTileFunction{}
				fakeTestFunc.Returns(nil)

				err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeTestFunc.CallCount()).To(Equal(1))

				ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
				Expect(ctx).NotTo(BeNil())
				Expect(w).NotTo(BeNil())

				Expect(configuration.Environment).To(Equal([]string{"PEAR=on-pizza"}))
			})
		})

		When("the using the long environment variable flag", func() {
			It("sets the metadata configuration flag", func() {
				args := []string{
					"--git-auth-token", "some-auth-token",
					"--environment-variable=PEAR=on-pizza",
				}

				fakeTestFunc := fakes.TestTileFunction{}
				fakeTestFunc.Returns(nil)

				err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeTestFunc.CallCount()).To(Equal(1))

				ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
				Expect(ctx).NotTo(BeNil())
				Expect(w).NotTo(BeNil())

				Expect(configuration.Environment).To(Equal([]string{"PEAR=on-pizza"}))
			})
		})
	})

	When("the git-auth-token flag is provided", func() {
		It("sets the GitAuthToken configuration flag", func() {
			args := []string{
				"--git-auth-token", "some-auth-token",
			}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, configuration := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(configuration.GitAuthToken).To(Equal("some-auth-token"))
		})
	})

	When("when the git-auth-token flag is not provided", func() {
		It("returns an error", func() {
			var args []string

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			Expect(err).To(MatchError(ContainSubstring("missing git auth token")))
		})
	})
})

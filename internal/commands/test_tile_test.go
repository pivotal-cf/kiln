package commands_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"github.com/pivotal-cf/kiln/internal/test"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
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
		output.Reset()
	})

	When("when no arguments are passed", func() {
		It("runs all the tests with initialized collaborators", func() {
			var emptySlice []string

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(emptySlice)
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

	AfterEach(func() {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		vendorDir := filepath.Join(filepath.Dir(filepath.Dir(wd)), "vendor")
		if info, err := os.Stat(vendorDir); err == nil && info.IsDir() { // no error
			_ = os.RemoveAll(vendorDir)
		}
	})

	When("when the tile directory does not exist", func() {
		It("runs all the tests with initalized collaborators", func() {
			dir, err := os.MkdirTemp("", "")
			Expect(err).NotTo(HaveOccurred())
			tilePath := filepath.Join(dir, "some-dir")

			args := []string{"--tile-path", tilePath}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err = commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(args)
			fmt.Println(err.Error())
			Expect(err).To(MatchError(ContainSubstring("failed to get information about --tile-path")))
		})
	})

	When("when the verbose flag argument is passed or the silent flag argument is not passed", func() {
		It("runs all the tests with initalized collaborators", func() {
			verboseFlagArgument := []string{"--verbose"}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(verboseFlagArgument)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, _ := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(output.String()).To(ContainSubstring("hello"))
		})
	})

	When("when the silent flag argument is passed", func() {
		It("runs all the tests without initalized collaborators", func() {
			silentFlagArgument := []string{"--silent"}

			fakeTestFunc := fakes.TestTileFunction{}
			fakeTestFunc.Returns(nil)
			fakeTestFunc.Stub = func(_ context.Context, w io.Writer, _ test.Configuration) error {
				_, _ = io.WriteString(w, "hello")
				return nil
			}

			err := commands.NewTileTestWithCollaborators(&output, fakeTestFunc.Spy).Execute(silentFlagArgument)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeTestFunc.CallCount()).To(Equal(1))

			ctx, w, _ := fakeTestFunc.ArgsForCall(0)
			Expect(ctx).NotTo(BeNil())
			Expect(w).NotTo(BeNil())

			Expect(output.String()).To(BeEmpty())
		})
	})

	When("when the manifest test is enabled", func() {
		It("it sets the RunManifest configuration flag", func() {
			args := []string{"--manifest"}

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

	When("when the migrations test is enabled", func() {
		It("it sets the RunMigrations configuration flag", func() {
			args := []string{"--migrations"}

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

	When("when the stability test is enabled", func() {
		It("it sets the RunMetadata configuration flag", func() {
			args := []string{"--stability"}

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

	When("when the stability test is enabled", func() {
		It("it sets the RunMetadata configuration flag", func() {
			args := []string{"--stability"}

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

	When("when ginkgo flag arguments are passed", func() {
		It("it sets the GinkgoFlags configuration", func() {
			args := []string{"--ginkgo-flags=peach pair"}

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

	When("when environment variables flags arguments are passed", func() {
		When("the using the short environment variable flag", func() {
			It("it sets the Environment configuration", func() {
				args := []string{"-e=PEAR=on-pizza"}

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
			It("it sets the Environment configuration", func() {
				args := []string{"--environment-variable=PEAR=on-pizza"}

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
})

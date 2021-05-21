package commands

import (
	"os"
	"testing"

	Ω "github.com/onsi/gomega"
)

func TestBake_loadFlags_sets_reasonable_defaults(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statNoError = func(string) (os.FileInfo, error) { return nil, nil }

		readFileCallCount = 0
		readFile          = func(name string) ([]byte, error) {
			readFileCallCount++
			switch name {
			case "version":
				return []byte("4.2.0"), nil
			default:
				return nil, os.ErrNotExist
			}
		}
	)

	err := bake.loadFlags([]string{}, statNoError, readFile)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal("Kilnfile"))
	please.Expect(bake.Options.Metadata).To(Ω.Equal("base.yml"))
	please.Expect(bake.Options.IconPath).To(Ω.Equal("icon.png"))
	please.Expect(bake.Options.OutputFile).To(Ω.Equal("tile-4.2.0.pivotal"))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.Equal([]string{"releases"}))
	please.Expect(bake.Options.FormDirectories).To(Ω.Equal([]string{"forms"}))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.Equal([]string{"instance_groups"}))
	please.Expect(bake.Options.JobDirectories).To(Ω.Equal([]string{"jobs"}))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.Equal([]string{"migrations"}))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.Equal([]string{"properties"}))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.Equal([]string{"runtime_configs"}))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.Equal([]string{"bosh_variables"}))

	please.Expect(readFileCallCount).To(Ω.Equal(1))
}

func TestBake_loadFlags_kilnfile_path_provided(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statNoError = func(string) (os.FileInfo, error) { return nil, nil }

		readFileCallCount = 0
		readFile          = func(name string) ([]byte, error) {
			readFileCallCount++
			switch name {
			case "version":
				return []byte("4.2.0"), nil
			default:
				return nil, os.ErrNotExist
			}
		}
	)

	err := bake.loadFlags([]string{
		"--kilnfile", "some-dir/Kilnfile",
		"--forms-directory", "do-not-change",
	}, statNoError, readFile)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal("some-dir/Kilnfile"))

	description := "it should prefix defaults with kiln path"
	please.Expect(bake.Options.Metadata).To(Ω.Equal("some-dir/base.yml"), description)
	please.Expect(bake.Options.IconPath).To(Ω.Equal("some-dir/icon.png"), description)
	please.Expect(bake.Options.OutputFile).To(Ω.Equal("tile-4.2.0.pivotal"))

	please.Expect(bake.Options.FormDirectories).To(Ω.Equal([]string{"do-not-change"}), "it should not prefix explicitly passed flags")

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.Equal([]string{"some-dir/releases"}), description)
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.Equal([]string{"some-dir/instance_groups"}), description)
	please.Expect(bake.Options.JobDirectories).To(Ω.Equal([]string{"some-dir/jobs"}), description)
	please.Expect(bake.Options.MigrationDirectories).To(Ω.Equal([]string{"some-dir/migrations"}), description)
	please.Expect(bake.Options.PropertyDirectories).To(Ω.Equal([]string{"some-dir/properties"}), description)
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.Equal([]string{"some-dir/runtime_configs"}), description)
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.Equal([]string{"some-dir/bosh_variables"}), description)

	please.Expect(readFileCallCount).To(Ω.Equal(1))
}

func TestBake_loadFlags_sets_empty_options_when_default_is_not_applicable(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statError = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		readError = func(name string) ([]byte, error) { return nil, os.ErrNotExist }
	)

	err := bake.loadFlags([]string{}, statError, readError)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal(""))
	please.Expect(bake.Options.Metadata).To(Ω.Equal(""))
	please.Expect(bake.Options.Version).To(Ω.Equal(""))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.FormDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.JobDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.HaveLen(0))
}

func TestBake_loadFlags_does_not_override_provided_flags(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statError = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

		readFileCallCount = 0
		readNotFound      = func(name string) ([]byte, error) {
			readFileCallCount++
			return nil, os.ErrNotExist
		}
	)

	err := bake.loadFlags([]string{
		"--kilnfile", "kilnfile",
		"--metadata", "metadata",
		"--icon", "icon",

		"--version", "4.2.0",
		"--output-file", "some-tile.pivotal",

		"--releases-directory", "releases-directory-1",
		"--releases-directory", "releases-directory-2",

		"--forms-directory", "forms-directory-1",
		"--forms-directory", "forms-directory-2",

		"--instance-groups-directory", "instance-groups-directory-1",
		"--instance-groups-directory", "instance-groups-directory-2",

		"--jobs-directory", "jobs-directory-1",
		"--jobs-directory", "jobs-directory-2",

		"--migrations-directory", "migrations-directory-1",
		"--migrations-directory", "migrations-directory-2",

		"--properties-directory", "properties-directory-1",
		"--properties-directory", "properties-directory-2",

		"--runtime-configs-directory", "runtime-configs-directory-1",
		"--runtime-configs-directory", "runtime-configs-directory-2",

		"--bosh-variables-directory", "bosh-variables-directory-1",
		"--bosh-variables-directory", "bosh-variables-directory-2",
	}, statError, readNotFound)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal("kilnfile"))
	please.Expect(bake.Options.Metadata).To(Ω.Equal("metadata"))
	please.Expect(bake.Options.IconPath).To(Ω.Equal("icon"))
	please.Expect(bake.Options.Version).To(Ω.Equal("4.2.0"))
	please.Expect(bake.Options.OutputFile).To(Ω.Equal("some-tile.pivotal"))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.Equal([]string{"releases-directory-1", "releases-directory-2"}))
	please.Expect(bake.Options.FormDirectories).To(Ω.Equal([]string{"forms-directory-1", "forms-directory-2"}))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.Equal([]string{"instance-groups-directory-1", "instance-groups-directory-2"}))
	please.Expect(bake.Options.JobDirectories).To(Ω.Equal([]string{"jobs-directory-1", "jobs-directory-2"}))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.Equal([]string{"migrations-directory-1", "migrations-directory-2"}))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.Equal([]string{"properties-directory-1", "properties-directory-2"}))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.Equal([]string{"runtime-configs-directory-1", "runtime-configs-directory-2"}))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.Equal([]string{"bosh-variables-directory-1", "bosh-variables-directory-2"}))

	please.Expect(readFileCallCount).To(Ω.Equal(0))
}

func TestBake_loadFlags_sets_default_output_file_if_not_set(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statNoError = func(string) (os.FileInfo, error) { return nil, nil }

		readFileCallCount = 0
		readFile          = func(name string) ([]byte, error) {
			readFileCallCount++
			switch name {
			case "version":
				return []byte("1.2.3"), nil
			default:
				return nil, os.ErrNotExist
			}
		}
	)

	err := bake.loadFlags([]string{}, statNoError, readFile)

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(bake.Options.OutputFile).To(Ω.Equal("tile-1.2.3.pivotal"))
	please.Expect(readFileCallCount).To(Ω.Equal(1))
}

func TestBake_loadFlags_does_not_set_outputs_file_when_meta_data_only_is_true(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statNoError    = func(string) (os.FileInfo, error) { return nil, nil }
		readFileError = func(name string) ([]byte, error) { return nil, os.ErrNotExist }
	)

	err := bake.loadFlags([]string{
		"--metadata-only",
	}, statNoError, readFileError)

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(bake.Options.OutputFile).To(Ω.Equal(""))
}

func TestBake_loadFlags_version_file_does_not_exist(t *testing.T) {
	please := Ω.NewWithT(t)

	var (
		bake Bake

		statError = func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}
		readFileError = func(filename string) ([]byte, error) {
			return nil, os.ErrNotExist
		}
	)

	err := bake.loadFlags([]string{}, statError, readFileError)

	please.Expect(err).NotTo(Ω.HaveOccurred(), "it should not return an error")
}

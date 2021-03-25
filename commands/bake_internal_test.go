package commands

import (
	"os"
	"testing"

	Ω "github.com/onsi/gomega"
)

func TestBake_loadFlags_sets_empty_options_when_default_path_does_not_exist(t *testing.T) {
	please := Ω.NewWithT(t)

	var bake Bake

	statReturnsErrorNotExists := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	err := bake.loadFlags([]string{}, statReturnsErrorNotExists)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal(""))
	please.Expect(bake.Options.Metadata).To(Ω.Equal(""))
	please.Expect(bake.Options.IconPath).To(Ω.Equal(""))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.FormDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.JobDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.HaveLen(0))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.HaveLen(0))
}

func TestBake_loadFlags_sets_provided_options(t *testing.T) {
	please := Ω.NewWithT(t)

	var bake Bake

	statReturnsErrorNotExists := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	err := bake.loadFlags([]string{
		"--kilnfile", "kilnfile",
		"--metadata", "metadata",
		"--icon", "icon",

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
	}, statReturnsErrorNotExists)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal("kilnfile"))
	please.Expect(bake.Options.Metadata).To(Ω.Equal("metadata"))
	please.Expect(bake.Options.IconPath).To(Ω.Equal("icon"))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.Equal([]string{"releases-directory-1", "releases-directory-2"}))
	please.Expect(bake.Options.FormDirectories).To(Ω.Equal([]string{"forms-directory-1", "forms-directory-2"}))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.Equal([]string{"instance-groups-directory-1", "instance-groups-directory-2"}))
	please.Expect(bake.Options.JobDirectories).To(Ω.Equal([]string{"jobs-directory-1", "jobs-directory-2"}))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.Equal([]string{"migrations-directory-1", "migrations-directory-2"}))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.Equal([]string{"properties-directory-1", "properties-directory-2"}))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.Equal([]string{"runtime-configs-directory-1", "runtime-configs-directory-2"}))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.Equal([]string{"bosh-variables-directory-1", "bosh-variables-directory-2"}))
}

func TestBake_loadFlags_sets_default_if_default_exists(t *testing.T) {
	please := Ω.NewWithT(t)

	var bake Bake

	fileIsFound := func(string) (os.FileInfo, error) { return nil, nil }

	err := bake.loadFlags([]string{}, fileIsFound)

	please.Expect(err).NotTo(Ω.HaveOccurred())

	please.Expect(bake.Options.Kilnfile).To(Ω.Equal("Kilnfile"))
	please.Expect(bake.Options.Metadata).To(Ω.Equal("base.yml"))
	please.Expect(bake.Options.IconPath).To(Ω.Equal("icon.png"))

	please.Expect(bake.Options.ReleaseDirectories).To(Ω.Equal([]string{"releases"}))
	please.Expect(bake.Options.FormDirectories).To(Ω.Equal([]string{"forms"}))
	please.Expect(bake.Options.InstanceGroupDirectories).To(Ω.Equal([]string{"instance_groups"}))
	please.Expect(bake.Options.JobDirectories).To(Ω.Equal([]string{"jobs"}))
	please.Expect(bake.Options.MigrationDirectories).To(Ω.Equal([]string{"migrations"}))
	please.Expect(bake.Options.PropertyDirectories).To(Ω.Equal([]string{"properties"}))
	please.Expect(bake.Options.RuntimeConfigDirectories).To(Ω.Equal([]string{"runtime_configs"}))
	please.Expect(bake.Options.BOSHVariableDirectories).To(Ω.Equal([]string{"bosh_variables"}))
}

func TestBake_loadFlags_sets_default_output_file_if_not_set(t *testing.T) {
	please := Ω.NewWithT(t)

	var bake Bake

	fileIsFound := func(string) (os.FileInfo, error) { return nil, nil }

	err := bake.loadFlags([]string{
		"--version", "1.2.3",
	}, fileIsFound)

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(bake.Options.OutputFile).To(Ω.Equal("tile-1.2.3.pivotal"))
}

func TestBake_loadFlagsAndDefaultsFromFiles_sets_version_option_if_flag_is_not_set(t *testing.T) {
	please := Ω.NewWithT(t)

	t.Run("version file exists", func(t *testing.T) {
		var bake Bake

		fileIsFound := func(string) (os.FileInfo, error) { return nil, nil }
		readFile := func(filename string) ([]byte, error) { return []byte("1.2.3\n"), nil }

		err := bake.loadFlagsAndDefaultsFromFiles([]string{}, fileIsFound, readFile)

		please.Expect(err).NotTo(Ω.HaveOccurred())
		please.Expect(bake.Options.Version).To(Ω.Equal("1.2.3"))
	})

	t.Run("version file does not exists", func(t *testing.T) {
		var bake Bake

		fileIsNotFound := func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		readFile := func(filename string) ([]byte, error) { return nil, os.ErrNotExist }

		err := bake.loadFlagsAndDefaultsFromFiles([]string{}, fileIsNotFound, readFile)

		please.Expect(err).To(Ω.MatchError("--version flag must be set"))
	})
}

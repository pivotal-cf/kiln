package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v2"
)

type Glaze struct {
	Options struct {
		Kilnfile string `short:"kf" long:"kilnfile" default:"Kilnfile"  description:"path to Kilnfile"`
	}
}

func (cmd *Glaze) Execute(args []string) error {
	_, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return err
	}

	kfPath := fullKilnfilePath(cmd.Options.Kilnfile)

	kilnfile, kilnfileLock, err := getKilnfiles(kfPath)
	if err != nil {
		return err
	}

	kilnfile, err = pinVersions(kilnfile, kilnfileLock)
	if err != nil {
		return err
	}

	return writeKilnfile(kilnfile, kfPath)
}

func (cmd *Glaze) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "This command locks all the components.",
		ShortDescription: "Pin versions in Kilnfile to match lock.",
	}
}

func fullKilnfilePath(path string) string {
	var kfPath = path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		kfPath = filepath.Join(path, "Kilnfile")
	}
	return kfPath
}

func getKilnfiles(path string) (cargo.Kilnfile, cargo.KilnfileLock, error) {
	kilnfile, err := getKilnfile(path)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}
	kilnfileLock, err := getKilnfileLock(path)
	if err != nil {
		return cargo.Kilnfile{}, cargo.KilnfileLock{}, err
	}

	return kilnfile, kilnfileLock, nil
}

func getKilnfile(path string) (cargo.Kilnfile, error) {
	kf, err := os.ReadFile(path)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("failed to read Kilnfile: %w", err)
	}

	var kilnfile cargo.Kilnfile
	err = yaml.Unmarshal(kf, &kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("failed to unmarshall Kilnfile: %w", err)
	}

	return kilnfile, nil
}

func getKilnfileLock(path string) (cargo.KilnfileLock, error) {
	kfl, err := os.ReadFile(path + ".lock")
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to read Kilnfile.lock: %w", err)
	}

	var kilnfileLock cargo.KilnfileLock
	err = yaml.Unmarshal(kfl, &kilnfileLock)
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to unmarshall Kilnfile.lock: %w", err)
	}

	return kilnfileLock, nil
}

func pinVersions(kf cargo.Kilnfile, kl cargo.KilnfileLock) (cargo.Kilnfile, error) {
	kf.Stemcell.Version = kl.Stemcell.Version
	for releaseIndex, release := range kf.Releases {
		l, err := kl.FindReleaseWithName(release.Name)
		if err != nil {
			return cargo.Kilnfile{}, fmt.Errorf("release with name %q not found in Kilnfile.lock: %w", release.Name, err)
		}

		kf.Releases[releaseIndex].Version = l.Version
	}

	return kf, nil
}

func writeKilnfile(kf cargo.Kilnfile, path string) error {
	kfBytes, err := yaml.Marshal(kf)
	if err != nil {
		return err
	}

	outputKilnfile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(outputKilnfile)
	_, err = outputKilnfile.Write(kfBytes)
	return err
}

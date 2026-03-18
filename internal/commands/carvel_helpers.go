package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/pivotal-cf/kiln/internal/baking"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/internal/component"
	"github.com/pivotal-cf/kiln/pkg/cargo"
	"gopkg.in/yaml.v3"
)

func loadKilnfileOnly(options flags.Standard) (cargo.Kilnfile, error) {
	fs := osfs.New("")
	variablesService := baking.NewTemplateVariablesService(fs)

	templateVariables, err := variablesService.FromPathsAndPairs(options.VariableFiles, options.Variables)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("failed to parse template variables: %w", err)
	}

	kilnfileFP, err := fs.Open(options.Kilnfile)
	if err != nil {
		return cargo.Kilnfile{}, fmt.Errorf("failed to open Kilnfile: %w", err)
	}
	defer func() { _ = kilnfileFP.Close() }()

	return cargo.InterpolateAndParseKilnfile(kilnfileFP, templateVariables)
}

func findArtifactorySource(kilnfile cargo.Kilnfile) (cargo.ReleaseSourceConfig, error) {
	for _, src := range kilnfile.ReleaseSources {
		if src.Type == cargo.BOSHReleaseTarballSourceTypeArtifactory {
			return src, nil
		}
	}
	return cargo.ReleaseSourceConfig{}, fmt.Errorf("no artifactory release source found in Kilnfile")
}

func downloadCarvelRelease(logger *log.Logger, kilnfile cargo.Kilnfile, lock cargo.KilnfileLock, destDir string) (string, error) {
	if len(lock.Releases) == 0 {
		return "", fmt.Errorf("Kilnfile.lock has no releases")
	}

	releaseLock := lock.Releases[0]
	sources := component.NewReleaseSourceRepo(kilnfile)

	logger.Printf("Downloading %s %s from %s", releaseLock.Name, releaseLock.Version, releaseLock.RemoteSource)
	local, err := sources.DownloadRelease(destDir, releaseLock)
	if err != nil {
		return "", fmt.Errorf("failed to download release: %w", err)
	}

	if releaseLock.SHA1 != "" && local.Lock.SHA1 != releaseLock.SHA1 {
		_ = os.Remove(local.LocalPath)
		return "", fmt.Errorf("downloaded release %q had incorrect SHA1 - expected %q, got %q", local.LocalPath, releaseLock.SHA1, local.Lock.SHA1)
	}

	return local.LocalPath, nil
}

func writeStandardKilnfileLock(lockfilePath string, releaseName, releaseVersion, remotePath, remoteSourceID, sha1 string) error {
	lock := cargo.KilnfileLock{
		Releases: []cargo.BOSHReleaseTarballLock{
			{
				Name:         releaseName,
				Version:      releaseVersion,
				RemotePath:   remotePath,
				RemoteSource: remoteSourceID,
				SHA1:         sha1,
			},
		},
		Stemcell: cargo.Stemcell{
			OS:      "ubuntu-jammy",
			Version: "1.446",
		},
	}

	data, err := yaml.Marshal(&lock)
	if err != nil {
		return fmt.Errorf("failed to marshal Kilnfile.lock: %w", err)
	}
	return os.WriteFile(lockfilePath, data, 0644)
}

func readStandardKilnfileLock(lockfilePath string) (cargo.KilnfileLock, error) {
	data, err := os.ReadFile(lockfilePath)
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to read Kilnfile.lock: %w", err)
	}
	var lock cargo.KilnfileLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to parse Kilnfile.lock: %w", err)
	}
	return lock, nil
}

func generateKilnfile(kilnfilePath, artifactoryHost, repo, username, password, pathTemplate string) error {
	if pathTemplate == "" {
		pathTemplate = "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz"
	}
	kf := cargo.Kilnfile{
		ReleaseSources: []cargo.ReleaseSourceConfig{
			{
				Type:            cargo.BOSHReleaseTarballSourceTypeArtifactory,
				ArtifactoryHost: artifactoryHost,
				Repo:            repo,
				Username:        username,
				Password:        password,
				PathTemplate:    pathTemplate,
			},
		},
	}

	data, err := yaml.Marshal(&kf)
	if err != nil {
		return fmt.Errorf("failed to marshal Kilnfile: %w", err)
	}
	return os.WriteFile(kilnfilePath, data, 0644)
}

func resolveKilnfilePath(kilnfilePath, sourcePath string) string {
	if kilnfilePath == "" || kilnfilePath == "Kilnfile" {
		return filepath.Join(sourcePath, "Kilnfile")
	}
	abs, err := filepath.Abs(kilnfilePath)
	if err != nil {
		return kilnfilePath
	}
	return abs
}

func resolveSourcePath(sourcePath string) (string, error) {
	if sourcePath == "" {
		var err error
		sourcePath, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		var err error
		sourcePath, err = filepath.Abs(sourcePath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve source directory: %w", err)
		}
	}
	return sourcePath, nil
}

package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/carvel/models"
)

type CarvelUpload struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelUploadOptions
}

type CarvelUploadOptions struct {
	SourceDirectory string `short:"s" long:"source-directory"     description:"path to the Carvel tile source directory (defaults to current directory)"`
	ArtifactoryHost string `          long:"artifactory-host"     description:"Artifactory server URL" required:"true"`
	ArtifactoryRepo string `          long:"artifactory-repo"     description:"Artifactory repository name" required:"true"`
	Username        string `short:"u" long:"artifactory-username" description:"Artifactory username" required:"true"`
	Password        string `short:"p" long:"artifactory-password" description:"Artifactory password or API key" required:"true"`
	PathTemplate    string `          long:"path-template"        description:"remote path template" default:"bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz"`
	OutputFile      string `short:"o" long:"output-file"          description:"also bake the tile to this path"`
	Verbose         bool   `short:"v" long:"verbose"              description:"enable verbose output"`
}

func NewCarvelUpload(outLogger, errLogger *log.Logger) CarvelUpload {
	return CarvelUpload{
		outLogger: outLogger,
		errLogger: errLogger,
	}
}

func (c CarvelUpload) Execute(args []string) error {
	_, err := jhanda.Parse(&c.Options, args)
	if err != nil {
		return err
	}

	sourcePath := c.Options.SourceDirectory
	if sourcePath == "" {
		sourcePath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		sourcePath, err = filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to resolve source directory: %w", err)
		}
	}

	baker := carvel.NewBaker()
	if c.Options.Verbose {
		baker.SetWriter(os.Stdout)
	}

	c.outLogger.Printf("Baking Carvel tile from %s", sourcePath)
	err = baker.Bake(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to prepare Carvel tile: %w", err)
	}

	tarball, err := baker.GetReleaseTarball()
	if err != nil {
		return fmt.Errorf("failed to locate release tarball: %w", err)
	}

	ver, err := baker.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get tile version: %w", err)
	}

	remotePath := fmt.Sprintf("bosh-releases/%s/%s-%s.tgz", baker.GetName(), baker.GetName(), ver)

	checksum, err := fileSHA256(tarball)
	if err != nil {
		return fmt.Errorf("failed to checksum release tarball: %w", err)
	}

	c.outLogger.Printf("Uploading %s to %s/%s/%s", filepath.Base(tarball), c.Options.ArtifactoryHost, c.Options.ArtifactoryRepo, remotePath)
	err = uploadToArtifactory(tarball, c.Options.ArtifactoryHost, c.Options.ArtifactoryRepo, remotePath, c.Options.Username, c.Options.Password)
	if err != nil {
		return fmt.Errorf("failed to upload to Artifactory: %w", err)
	}

	lockfilePath := filepath.Join(sourcePath, "Kilnfile.lock")
	lf := models.CarvelLockfile{
		Release: models.CarvelReleaseLock{
			Name:       baker.GetName(),
			Version:    ver,
			RemotePath: remotePath,
			SHA256:     checksum,
		},
	}
	err = lf.WriteFile(lockfilePath)
	if err != nil {
		return fmt.Errorf("failed to write lockfile: %w", err)
	}
	c.outLogger.Printf("Updated %s", lockfilePath)

	if c.Options.OutputFile != "" {
		targetPath, err := filepath.Abs(c.Options.OutputFile)
		if err != nil {
			return fmt.Errorf("failed to resolve output file path: %w", err)
		}
		err = baker.KilnBake(targetPath)
		if err != nil {
			return fmt.Errorf("failed to bake tile: %w", err)
		}
		c.outLogger.Printf("Baked %s version %s to %s", baker.GetName(), ver, targetPath)
	}

	return nil
}

func (c CarvelUpload) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "Generates a BOSH release from a Carvel tile source, uploads the release tarball to Artifactory, and updates Kilnfile.lock with the remote location and checksum.",
		ShortDescription: "uploads a Carvel BOSH release to Artifactory",
		Flags:            c.Options,
	}
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func uploadToArtifactory(localPath, host, repo, remotePath, username, password string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	uploadURL := host + "/" + repo + "/" + remotePath

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return err
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/gzip")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

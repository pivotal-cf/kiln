package commands

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pivotal-cf/jhanda"
	"github.com/pivotal-cf/kiln/internal/carvel"
	"github.com/pivotal-cf/kiln/internal/commands/flags"
	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type CarvelUpload struct {
	outLogger *log.Logger
	errLogger *log.Logger
	Options   CarvelUploadOptions
}

type CarvelUploadOptions struct {
	flags.Standard
	SourceDirectory string `short:"s" long:"source-directory" description:"path to the Carvel tile source directory (defaults to current directory)"`
	OutputFile      string `short:"o" long:"output-file"      description:"also bake the tile to this path"`
	PathTemplate    string `          long:"path-template"     description:"remote path template override" default:"bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz"`
	Verbose         bool   `short:"v" long:"verbose"           description:"enable verbose output"`
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

	sourcePath, err := resolveSourcePath(c.Options.SourceDirectory)
	if err != nil {
		return err
	}

	kilnfilePath := resolveKilnfilePath(c.Options.Kilnfile, sourcePath)

	if _, statErr := os.Stat(kilnfilePath); statErr != nil {
		return fmt.Errorf("could not find Kilnfile at %s: create a Kilnfile with an artifactory release_source", kilnfilePath)
	}

	c.Options.Kilnfile = kilnfilePath
	kilnfile, err := loadKilnfileOnly(c.Options.Standard)
	if err != nil {
		return fmt.Errorf("failed to load Kilnfile: %w", err)
	}

	artConfig, err := findArtifactorySource(kilnfile)
	if err != nil {
		return err
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

	pathTmpl := c.Options.PathTemplate
	if artConfig.PathTemplate != "" {
		pathTmpl = artConfig.PathTemplate
	}
	remotePath, err := evaluatePathTemplate(pathTmpl, baker.GetName(), ver)
	if err != nil {
		return fmt.Errorf("failed to evaluate path template: %w", err)
	}

	sha1sum, err := fileSHA1(tarball)
	if err != nil {
		return fmt.Errorf("failed to checksum release tarball: %w", err)
	}

	c.outLogger.Printf("Uploading %s to %s/%s/%s", filepath.Base(tarball), artConfig.ArtifactoryHost, artConfig.Repo, remotePath)
	err = uploadToArtifactory(tarball, artConfig.ArtifactoryHost, artConfig.Repo, remotePath, artConfig.Username, artConfig.Password)
	if err != nil {
		return fmt.Errorf("failed to upload to Artifactory: %w", err)
	}

	sourceID := cargo.BOSHReleaseTarballSourceID(artConfig)
	lockfilePath := kilnfilePath + ".lock"
	err = writeStandardKilnfileLock(lockfilePath, baker.GetName(), ver, remotePath, sourceID, sha1sum)
	if err != nil {
		return fmt.Errorf("failed to write Kilnfile.lock: %w", err)
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
		Description:      "Generates a BOSH release from a Carvel tile source, uploads the release tarball to Artifactory, and updates Kilnfile.lock with the remote location and checksum. Artifactory credentials are read from the Kilnfile's release_sources (typically interpolated from ~/.kiln/credentials.yml).",
		ShortDescription: "uploads a Carvel BOSH release to Artifactory",
		Flags:            c.Options,
	}
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func evaluatePathTemplate(tmpl, name, version string) (string, error) {
	t, err := template.New("path").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, cargo.BOSHReleaseTarballSpecification{Name: name, Version: version})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
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

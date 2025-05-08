package bake

import (
	"archive/zip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/crhntr/yamlutil/yamlnode"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/tile"
)

const (
	fileMode = 0644
	dirMode  = 0744

	gitKeepFilename  = ".gitkeep"
	defaultGitIgnore =
	/* language=gitignore */ `
*.pivotal
releases/*.tgz
releases/*.tar.gz
*.out
`
)

func New(outputDirectory, tilePath string, spec cargo.Kilnfile) error {
	f, openErr := os.Open(tilePath)
	info, statErr := os.Stat(tilePath)
	if err := errors.Join(statErr, openErr); err != nil {
		return err
	}
	defer closeAndIgnoreError(f)
	return newFromReader(outputDirectory, f, info.Size(), spec)
}

func newFromReader(outputDirectory string, r io.ReaderAt, size int64, spec cargo.Kilnfile) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	return newFromFS(outputDirectory, zr, spec)
}

func newFromFS(outputDirectory string, dir fs.FS, spec cargo.Kilnfile) error {
	productTemplateBuffer, err := tile.ReadMetadataFromFS(dir)
	if err != nil {
		return err
	}
	productTemplate, err := newFromProductTemplate(outputDirectory, productTemplateBuffer)
	if err != nil {
		return err
	}
	if err := extractMigrations(outputDirectory, dir); err != nil {
		return err
	}
	releaseLocks, err := extractReleases(outputDirectory, productTemplate, dir)
	if err != nil {
		return err
	}
	if err := newKilnfiles(outputDirectory, spec, releaseLocks); err != nil {
		return err
	}
	baseYML, err := yaml.Marshal(productTemplate)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDirectory, DefaultFilepathBaseYML), baseYML, fileMode); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outputDirectory, ".gitignore"), []byte(defaultGitIgnore), fileMode); err != nil {
		return err
	}
	return nil
}

func newKilnfiles(outputDirectory string, spec cargo.Kilnfile, releaseTarballs []cargo.BOSHReleaseTarball) error {
	var lock cargo.KilnfileLock
	for _, tarball := range releaseTarballs {
		lock.Releases = append(lock.Releases, cargo.BOSHReleaseTarballLock{
			Name:    tarball.Manifest.Name,
			Version: tarball.Manifest.Version,
			SHA1:    tarball.SHA1,
		})
	}
	slices.SortFunc(lock.Releases, func(a, b cargo.BOSHReleaseTarballLock) int {
		return strings.Compare(a.Name, b.Name)
	})
	spec.Releases = spec.Releases[:0]
	for _, lock := range spec.Releases {
		spec.Releases = append(spec.Releases, cargo.BOSHReleaseTarballSpecification{
			Name:            lock.Name,
			Version:         lock.Version,
			DeGlazeBehavior: cargo.LockPatch,
			FloatAlways:     false,
		})
	}
	lockBuf, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	specBuf, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	return errors.Join(
		os.WriteFile(filepath.Join(outputDirectory, DefaultFilepathKilnfileLock), lockBuf, fileMode),
		os.WriteFile(filepath.Join(outputDirectory, DefaultFilepathKilnfile), specBuf, fileMode),
	)
}

func newFromProductTemplate(outputDirectory string, productTemplate []byte) (*yaml.Node, error) {
	var productTemplateNode yaml.Node
	if err := yaml.Unmarshal(productTemplate, &productTemplateNode); err != nil {
		return &productTemplateNode, fmt.Errorf("failed to parse product template: %w", err)
	}
	return &productTemplateNode, errors.Join(writeIconPNG(outputDirectory, &productTemplateNode))
}

func extractMigrations(outputDirectory string, dir fs.FS) error {
	migrations, err := fs.Glob(dir, "migrations/*.js")
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		outPath := filepath.Join(outputDirectory, filepath.FromSlash(migration))
		if err := copyFile(outPath, dir, migration); err != nil {
			return err

		}
	}
	return nil
}

func extractReleases(outputDirectory string, productTemplate *yaml.Node, dir fs.FS) ([]cargo.BOSHReleaseTarball, error) {
	releases, err := fs.Glob(dir, "releases/*.tgz")
	if err != nil {
		return nil, err
	}
	var tarballs []cargo.BOSHReleaseTarball

	if err := os.MkdirAll(filepath.Join(outputDirectory, DefaultDirectoryReleases), dirMode); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(outputDirectory, DefaultDirectoryReleases, gitKeepFilename), nil, fileMode); err != nil {
		return nil, err
	}
	for _, release := range releases {
		outPath := filepath.Join(outputDirectory, filepath.FromSlash(release))
		if err := copyFile(outPath, dir, release); err != nil {
			return nil, err
		}
		releaseTarball, err := cargo.OpenBOSHReleaseTarball(outPath)
		if err != nil {
			return nil, err
		}
		tarballs = append(tarballs, releaseTarball)
	}
	releasesNode, found := yamlnode.LookupKey(productTemplate, "releases")
	if !found {
		return nil, err
	}
	var releasesList []string
	for _, tarball := range tarballs {
		releasesList = append(releasesList, fmt.Sprintf("$( release %q )", tarball.Manifest.Name))
	}
	return tarballs, releasesNode.Encode(&releasesList)
}

func copyFile(out string, dir fs.FS, p string) error {
	srcFile, err := dir.Open(p)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(srcFile)

	dstFile, err := os.Create(out)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(dstFile)

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func writeIconPNG(outputDirectory string, productTemplate *yaml.Node) error {
	iconImageNode, found := yamlnode.LookupKey(productTemplate, "icon_image")
	if !found {
		return fmt.Errorf("icon_image not found in product template")
	}
	iconImage, err := base64.StdEncoding.DecodeString(strings.TrimSpace(iconImageNode.Value))
	if err != nil {
		return fmt.Errorf("failed to decode icon_image: %w", err)
	}
	iconImageNode.Value = `$( icon )`
	return os.WriteFile(filepath.Join(outputDirectory, DefaultFilepathIconImage), iconImage, fileMode)
}

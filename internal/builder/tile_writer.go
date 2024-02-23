package builder

import (
	"bytes"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type TileWriter struct {
	filesystem filesystem
	zipper     zipper
	logger     logger
}

//counterfeiter:generate -o ./fakes/filesystem.go --fake-name Filesystem . filesystem
type filesystem interface {
	Create(path string) (io.WriteCloser, error)
	Open(path string) (io.ReadCloser, error)
	Walk(root string, walkFn filepath.WalkFunc) error
	Remove(path string) error
}

//counterfeiter:generate -o ./fakes/zipper.go --fake-name Zipper . zipper

type zipper interface {
	SetWriter(writer io.Writer)
	SetModified(t time.Time)
	Add(path string, file io.Reader) error
	AddWithMode(path string, file io.Reader, mode os.FileMode) error
	CreateFolder(path string) error
	Close() error
}

//counterfeiter:generate -o ./fakes/file_info.go --fake-name FileInfo os.FileInfo

func NewTileWriter(filesystem filesystem, zipper zipper, logger logger) TileWriter {
	return TileWriter{
		filesystem: filesystem,
		zipper:     zipper,
		logger:     logger,
	}
}

type WriteInput struct {
	OutputFile           string
	StubReleases         bool
	MigrationDirectories []string
	ReleaseDirectories   []string
	EmbedPaths           []string
	ModTime              time.Time
}

type tileMetadata struct {
	Releases []release `yaml:"releases"`
}

type release struct {
	File string `yaml:"file"`
}

func (w TileWriter) Write(generatedMetadataContents []byte, input WriteInput) error {
	w.logger.Printf("Building %s...", input.OutputFile)

	f, err := w.filesystem.Create(input.OutputFile)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)

	w.zipper.SetWriter(f)
	w.zipper.SetModified(input.ModTime)

	err = w.addToZipper(path.Join("metadata", "metadata.yml"), bytes.NewBuffer(generatedMetadataContents), input.OutputFile)
	if err != nil {
		w.removeOutputFile(input.OutputFile)
		return err
	}

	err = w.addMigrations(input.MigrationDirectories, input.OutputFile)
	if err != nil {
		w.removeOutputFile(input.OutputFile)
		return err
	}

	if input.StubReleases {
		err = w.addStubReleases(generatedMetadataContents, input.OutputFile)
	} else {
		err = w.addReleases(input.ReleaseDirectories, input.OutputFile)
	}
	if err != nil {
		w.removeOutputFile(input.OutputFile)
		return err
	}

	err = w.addEmbeddedPaths(input.EmbedPaths, input.OutputFile)
	if err != nil {
		w.removeOutputFile(input.OutputFile)
		return err
	}

	err = w.zipper.Close()
	if err != nil {
		w.removeOutputFile(input.OutputFile)
		return err
	}

	return nil
}

func (w TileWriter) addReleases(releasesDirs []string, outputFile string) error {
	for _, releasesDirectory := range releasesDirs {
		err := w.addReleaseTarballs(releasesDirectory, outputFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w TileWriter) addStubReleases(generatedMetadataContents []byte, outputFile string) error {
	var metadata tileMetadata
	err := yaml.Unmarshal(generatedMetadataContents, &metadata)
	if err != nil {
		return err
	}
	for _, release := range metadata.Releases {
		rp := path.Join("releases", release.File)
		contents := io.NopCloser(strings.NewReader(""))
		err = w.addToZipper(rp, contents, outputFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w TileWriter) addReleaseTarballs(releasesDir string, outputFile string) error {
	return w.filesystem.Walk(releasesDir, func(filePath string, info os.FileInfo, err error) error {
		isTarball, _ := regexp.MatchString("tgz$|tar.gz$", filePath)
		if !isTarball {
			return nil
		}

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := w.filesystem.Open(filePath)
		if err != nil {
			return err
		}
		defer closeAndIgnoreError(file)

		return w.addToZipper(path.Join("releases", filepath.Base(filePath)), file, outputFile)
	})
}

func (w TileWriter) addEmbeddedPaths(embedPaths []string, outputFile string) error {
	for _, embedPath := range embedPaths {
		err := w.addEmbeddedPath(embedPath, outputFile)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w TileWriter) addEmbeddedPath(pathToEmbed, outputFile string) error {
	return w.filesystem.Walk(pathToEmbed, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := w.filesystem.Open(filePath)
		if err != nil {
			return err
		}
		defer closeAndIgnoreError(file)

		relativePath, err := filepath.Rel(pathToEmbed, filePath)
		if err != nil {
			return err // not tested
		}

		entryPath := path.Join("embed", filepath.Join(filepath.Base(pathToEmbed), relativePath))
		return w.addToZipperWithMode(entryPath, file, info.Mode(), outputFile)
	})
}

func (w TileWriter) addMigrations(migrationsDir []string, outputFile string) error {
	var found bool

	for _, migrationDir := range migrationsDir {
		err := w.filesystem.Walk(migrationDir, func(filePath string, info os.FileInfo, err error) error {
			isNodeFile, _ := regexp.MatchString(`node_modules/`, filePath)
			isTest, _ := regexp.MatchString(`tests/`, filePath)
			isJsFile, _ := regexp.MatchString(`.js$`, filePath)

			if isNodeFile || isTest || !isJsFile {
				return nil
			}

			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			found = true

			file, err := w.filesystem.Open(filePath)
			if err != nil {
				return err
			}
			defer closeAndIgnoreError(file)

			return w.addToZipper(path.Join("migrations", "v1", filepath.Base(filePath)), file, outputFile)
		})
		if err != nil {
			return err
		}
	}

	if !found {
		return w.addEmptyMigrationsDirectory(outputFile)
	}

	return nil
}

func (w TileWriter) addToZipper(path string, contents io.Reader, outputFile string) error {
	w.logger.Printf("Adding %s to %s...", path, outputFile)

	return w.zipper.Add(path, contents)
}

func (w TileWriter) addToZipperWithMode(path string, contents io.Reader, mode os.FileMode, outputFile string) error {
	w.logger.Printf("Adding %s to %s...", path, outputFile)

	return w.zipper.AddWithMode(path, contents, mode)
}

func (w TileWriter) addEmptyMigrationsDirectory(outputFile string) error {
	w.logger.Printf("Creating empty migrations folder in %s...", outputFile)
	err := w.zipper.CreateFolder(path.Join("migrations", "v1"))
	if err != nil {
		return err
	}
	return nil
}

func (w TileWriter) removeOutputFile(path string) {
	err := w.filesystem.Remove(path)
	if err != nil {
		w.logger.Printf("failed cleaning up zip %q: %s", path, err.Error())
	}
}

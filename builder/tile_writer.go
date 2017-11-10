package builder

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pivotal-cf/kiln/commands"
)

type TileWriter struct {
	filesystem       filesystem
	zipper           zipper
	logger           logger
	md5SumCalculator md5SumCalculator
}

//go:generate counterfeiter -o ./fakes/filesystem.go --fake-name Filesystem . filesystem

type filesystem interface {
	Open(name string) (io.ReadWriteCloser, error)
	Walk(root string, walkFn filepath.WalkFunc) error
}

type md5SumCalculator interface {
	Checksum(path string) (string, error)
}

//go:generate counterfeiter -o ./fakes/zipper.go --fake-name Zipper . zipper

type zipper interface {
	SetPath(path string) error
	Add(path string, file io.Reader) error
	AddWithMode(path string, file io.Reader, mode os.FileMode) error
	CreateFolder(path string) error
	Close() error
}

//go:generate counterfeiter -o ./fakes/file_info.go --fake-name FileInfo . fileinfo

type fileinfo interface {
	Name() string
	Size() int64
	Mode() os.FileMode
	ModTime() time.Time
	IsDir() bool
	Sys() interface{}
}

func NewTileWriter(filesystem filesystem, zipper zipper, logger logger, md5SumCalculator md5SumCalculator) TileWriter {
	return TileWriter{
		filesystem:       filesystem,
		zipper:           zipper,
		logger:           logger,
		md5SumCalculator: md5SumCalculator,
	}
}

func (w TileWriter) Write(generatedMetadataContents []byte, config commands.BakeConfig) error {
	w.logger.Printf("Building %s...", config.OutputFile)

	err := w.zipper.SetPath(config.OutputFile)
	if err != nil {
		return err
	}

	err = w.addToZipper(filepath.Join("metadata", fmt.Sprintf("%s.yml", config.ProductName)), bytes.NewBuffer(generatedMetadataContents), config.OutputFile)
	if err != nil {
		return err
	}

	err = w.addMigrations(config.MigrationDirectories, config.OutputFile)
	if err != nil {
		return err
	}

	if len(config.ReleaseDirectories) > 0 {
		for _, releasesDirectory := range config.ReleaseDirectories {
			err = w.addReleaseTarballs(releasesDirectory, config.StubReleases, config.OutputFile)
			if err != nil {
				return err
			}
		}
	}

	for _, embedPath := range config.EmbedPaths {
		err = w.addEmbeddedPath(embedPath, config.OutputFile)
		if err != nil {
			return err
		}
	}

	err = w.zipper.Close()
	if err != nil {
		return err
	}

	w.logger.Printf("Calculating md5 sum of %s...", config.OutputFile)
	md5Sum, err := w.md5SumCalculator.Checksum(config.OutputFile)
	if err != nil {
		return err
	}

	w.logger.Printf("Calculated md5 sum: %s", md5Sum)

	return nil
}

func (w TileWriter) addReleaseTarballs(releasesDir string, stubReleases bool, outputFile string) error {
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

		var file io.ReadCloser = ioutil.NopCloser(strings.NewReader(""))
		if !stubReleases {
			file, err = w.filesystem.Open(filePath)
			if err != nil {
				return err
			}
			defer file.Close()
		}

		return w.addToZipper(filepath.Join("releases", filepath.Base(filePath)), file, outputFile)
	})
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
		defer file.Close()

		relativePath, err := filepath.Rel(pathToEmbed, filePath)
		if err != nil {
			return err //not tested
		}

		entryPath := filepath.Join("embed", filepath.Join(filepath.Base(pathToEmbed), relativePath))
		return w.addToZipperWithMode(entryPath, file, info.Mode(), outputFile)
	})
}

func (w TileWriter) addMigrations(migrationsDir []string, outputFile string) error {
	var found bool

	for _, migrationDir := range migrationsDir {
		err := w.filesystem.Walk(migrationDir, func(filePath string, info os.FileInfo, err error) error {
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
			defer file.Close()

			return w.addToZipper(filepath.Join("migrations", "v1", filepath.Base(filePath)), file, outputFile)
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

func (w TileWriter) containsMigrations(entries []string) bool {
	migrationsPrefix := filepath.Join("migrations", "v1")
	for _, entry := range entries {
		if strings.HasPrefix(entry, migrationsPrefix) {
			return true
		}
	}
	return false
}

func (w TileWriter) addEmptyMigrationsDirectory(outputFile string) error {
	w.logger.Printf("Creating empty migrations folder in %s...", outputFile)
	err := w.zipper.CreateFolder(filepath.Join("migrations", "v1"))
	if err != nil {
		return err
	}
	return nil
}

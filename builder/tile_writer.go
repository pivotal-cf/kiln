package builder

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

type zipper interface {
	SetPath(path string) error
	Add(path string, file io.Reader) error
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

	files := map[string]io.Reader{}

	files[filepath.Join("metadata", fmt.Sprintf("%s.yml", config.ProductName))] = bytes.NewBuffer(generatedMetadataContents)

	if len(config.ReleaseDirectories) > 0 {
		for _, releasesDirectory := range config.ReleaseDirectories {
			err = w.addReleaseTarballs(files, releasesDirectory, config.StubReleases)
			if err != nil {
				return err
			}
		}
	}

	if len(config.MigrationDirectories) > 0 {
		for _, migrationsDir := range config.MigrationDirectories {
			err = w.addMigrations(files, migrationsDir)
			if err != nil {
				return err
			}
		}
	}

	for _, embedPath := range config.EmbedPaths {
		err = w.addEmbeddedPath(files, embedPath)
		if err != nil {
			return err
		}
	}

	var paths []string
	for path := range files {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	if !w.containsMigrations(paths) {
		err = w.addEmptyMigrationsDirectory(config.OutputFile)
		if err != nil {
			return err
		}
	}

	for _, path := range paths {
		w.logger.Printf("Adding %s to %s...", path, config.OutputFile)

		err := w.zipper.Add(path, files[path])
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

func (w TileWriter) addReleaseTarballs(files map[string]io.Reader, releasesDir string, stubReleases bool) error {
	return w.filesystem.Walk(releasesDir, func(filePath string, info os.FileInfo, err error) error {
		var file io.Reader = strings.NewReader("")

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !stubReleases {
			file, err = w.filesystem.Open(filePath)
			if err != nil {
				return err
			}
		}
		files[filepath.Join("releases", filepath.Base(filePath))] = file

		return nil
	})
}

func (w TileWriter) addEmbeddedPath(files map[string]io.Reader, pathToEmbed string) error {
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

		relativePath, err := filepath.Rel(pathToEmbed, filePath)
		if err != nil {
			return err //not tested
		}

		entryPath := filepath.Join("embed", filepath.Join(filepath.Base(pathToEmbed), relativePath))

		files[entryPath] = file
		return nil
	})
}

func (w TileWriter) addMigrations(files map[string]io.Reader, migrationsDir string) error {
	return w.filesystem.Walk(migrationsDir, func(filePath string, info os.FileInfo, err error) error {
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

		files[filepath.Join("migrations", "v1", filepath.Base(filePath))] = file
		return nil
	})
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

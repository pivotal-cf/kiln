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
)

type TileWriter struct {
	filesystem              filesystem
	zipper                  zipper
	logger                  logger
	md5SumCalculator        md5SumCalculator
	contentMigrationBuilder contentMigrationBuilder
}

type WriteConfig struct {
	ReleaseTarballs      []string
	Migrations           []string
	MigrationsDirectory  string
	ContentMigrations    []string
	BaseContentMigration string
	StemcellTarball      string
	Handcraft            string
	Version              string
	FinalVersion         string
	ProductName          string
	FilenamePrefix       string
	OutputDir            string
	StubReleases         bool
}

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

type contentMigrationBuilder interface {
	Build(baseContentMigration string, version string, contentMigrations []string) ([]byte, error)
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

func NewTileWriter(filesystem filesystem, zipper zipper, contentMigrationBuilder contentMigrationBuilder, logger logger, md5SumCalculator md5SumCalculator) TileWriter {
	return TileWriter{
		filesystem:              filesystem,
		zipper:                  zipper,
		logger:                  logger,
		md5SumCalculator:        md5SumCalculator,
		contentMigrationBuilder: contentMigrationBuilder,
	}
}

func (w TileWriter) Write(metadataContents []byte, writeCfg WriteConfig) error {
	w.logger.Println("Building .pivotal file...")

	tileFileName := filepath.Join(writeCfg.OutputDir, fmt.Sprintf("%s-%s.pivotal", writeCfg.FilenamePrefix, writeCfg.Version))

	err := w.zipper.SetPath(tileFileName)
	if err != nil {
		return err
	}

	files := map[string]io.Reader{}

	files[filepath.Join("metadata", fmt.Sprintf("%s.yml", writeCfg.ProductName))] = bytes.NewBuffer(metadataContents)

	err = w.addReleaseTarballs(files, writeCfg.ReleaseTarballs, writeCfg.StubReleases)
	if err != nil {
		return err
	}

	if writeCfg.MigrationsDirectory != "" {
		err = w.addMigrations(files, writeCfg.MigrationsDirectory)
		if err != nil {
			return err
		}
	}

	if len(writeCfg.ContentMigrations) > 0 {
		contentMigrationsContents, err := w.contentMigrationBuilder.Build(
			writeCfg.BaseContentMigration,
			writeCfg.FinalVersion,
			writeCfg.ContentMigrations)

		if err != nil {
			return err
		}

		files[filepath.Join("content_migrations", "migrations.yml")] = bytes.NewBuffer(contentMigrationsContents)
	}

	var paths []string
	for path := range files {
		paths = append(paths, path)
	}

	sort.Strings(paths)

	if !w.containsMigrations(paths) {
		err = w.addEmptyMigrationsDirectory()
		if err != nil {
			return err
		}
	}

	for _, path := range paths {
		w.logger.Printf("Adding %s to .pivotal...", path)

		err := w.zipper.Add(path, files[path])
		if err != nil {
			return err
		}
	}

	err = w.zipper.Close()
	if err != nil {
		return err
	}

	w.logger.Println("Calculating md5 sum of .pivotal...")
	md5Sum, err := w.md5SumCalculator.Checksum(tileFileName)
	if err != nil {
		return err
	}

	w.logger.Printf("Calculated md5 sum: %s", md5Sum)

	return nil
}

func (w TileWriter) addReleaseTarballs(files map[string]io.Reader, releaseTarballs []string, stubReleases bool) error {
	for _, r := range releaseTarballs {
		var file io.Reader = strings.NewReader("")
		var err error

		if !stubReleases {
			file, err = w.filesystem.Open(r)
			if err != nil {
				return err
			}
		}

		files[filepath.Join("releases", filepath.Base(r))] = file
	}
	return nil
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

func (w TileWriter) addEmptyMigrationsDirectory() error {
	w.logger.Printf("Creating empty migrations folder in .pivotal...")
	err := w.zipper.CreateFolder(filepath.Join("migrations", "v1"))
	if err != nil {
		return err
	}
	return nil
}

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

func (e TileWriter) Write(metadataContents []byte, writeCfg WriteConfig) error {
	e.logger.Println("Building .pivotal file...")

	tileFileName := filepath.Join(writeCfg.OutputDir, fmt.Sprintf("%s-%s.pivotal", writeCfg.FilenamePrefix, writeCfg.Version))

	err := e.zipper.SetPath(tileFileName)
	if err != nil {
		return err
	}

	files := map[string]io.Reader{}

	files[filepath.Join("metadata", fmt.Sprintf("%s.yml", writeCfg.ProductName))] = bytes.NewBuffer(metadataContents)

	for _, r := range writeCfg.ReleaseTarballs {
		var file io.Reader = strings.NewReader("")
		var err error

		if !writeCfg.StubReleases {
			file, err = e.filesystem.Open(r)
			if err != nil {
				return err
			}
		}

		files[filepath.Join("releases", filepath.Base(r))] = file
	}

	if writeCfg.MigrationsDirectory != "" {
		err = e.filesystem.Walk(writeCfg.MigrationsDirectory, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := e.filesystem.Open(filePath)
			if err != nil {
				return err
			}

			files[filepath.Join("migrations", "v1", filepath.Base(filePath))] = file
			return nil
		})

		if err != nil {
			return err
		}
	}

	if len(writeCfg.ContentMigrations) > 0 {
		contentMigrationsContents, err := e.contentMigrationBuilder.Build(writeCfg.BaseContentMigration, writeCfg.FinalVersion, writeCfg.ContentMigrations)
		if err != nil {
			return err
		}

		files[filepath.Join("content_migrations", "migrations.yml")] = bytes.NewBuffer(contentMigrationsContents)
	}

	var keys []string
	for key := range files {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	err = e.ensureMigrationsDirPresent(keys)
	if err != nil {
		return err
	}

	for _, path := range keys {
		e.logger.Printf("Adding %s to .pivotal...", path)

		err := e.zipper.Add(path, files[path])
		if err != nil {
			return err
		}
	}

	err = e.zipper.Close()
	if err != nil {
		return err
	}

	e.logger.Println("Calculating md5 sum of .pivotal...")
	md5Sum, err := e.md5SumCalculator.Checksum(tileFileName)
	if err != nil {
		return err
	}

	e.logger.Printf("Calculated md5 sum: %s", md5Sum)

	return nil
}

func (e TileWriter) ensureMigrationsDirPresent(filenames []string) error {
	migrationsPrefix := filepath.Join("migrations", "v1")
	for _, f := range filenames {
		if strings.HasPrefix(f, migrationsPrefix) {
			return nil
		}
	}
	e.logger.Printf("Creating empty migrations folder in .pivotal...")
	err := e.zipper.CreateFolder(filepath.Join("migrations", "v1"))
	if err != nil {
		return err
	}
	return nil
}

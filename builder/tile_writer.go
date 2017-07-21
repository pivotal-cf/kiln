package builder

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
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
	ContentMigrations    []string
	BaseContentMigration string
	StemcellTarball      string
	Handcraft            string
	Version              string
	FinalVersion         string
	Name                 string
	OutputDir            string
	StubReleases         bool
}

type filesystem interface {
	Open(name string) (io.ReadWriteCloser, error)
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

	tileFileName := filepath.Join(writeCfg.OutputDir, fmt.Sprintf("%s-%s.pivotal", writeCfg.Name, writeCfg.Version))

	err := e.zipper.SetPath(tileFileName)
	if err != nil {
		return err
	}

	files := map[string]io.Reader{}

	files[filepath.Join("metadata", fmt.Sprintf("%s.yml", writeCfg.Name))] = bytes.NewBuffer(metadataContents)

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

	for _, m := range writeCfg.Migrations {
		file, err := e.filesystem.Open(m)
		if err != nil {
			return err
		}

		files[filepath.Join("migrations", "v1", filepath.Base(m))] = file
	}

	if len(writeCfg.Migrations) == 0 {
		e.logger.Printf("Creating empty migrations folder in .pivotal...")
		err := e.zipper.CreateFolder(filepath.Join("migrations", "v1"))
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

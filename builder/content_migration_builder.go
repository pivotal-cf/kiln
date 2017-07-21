package builder

import (
	"io/ioutil"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v2"
)

type ContentMigrationBuilder struct {
	logger logger
}

type ContentMigrations struct {
	Product                   string        `yaml:"product"`
	InstallationSchemaVersion string        `yaml:"installation_schema_version"`
	ToVersion                 string        `yaml:"to_version"`
	Migrations                []interface{} `yaml:"migrations"`
}

func NewContentMigrationBuilder(logger logger) ContentMigrationBuilder {
	return ContentMigrationBuilder{
		logger: logger,
	}
}

func (c ContentMigrationBuilder) Build(baseContentMigrationFilepath, version string, inputContentMigrations []string) ([]byte, error) {
	baseContentMigrationContent, err := ioutil.ReadFile(baseContentMigrationFilepath)
	if err != nil {
		return []byte{}, err
	}

	var contentMigrations ContentMigrations
	err = yaml.Unmarshal(baseContentMigrationContent, &contentMigrations)
	if err != nil {
		return []byte{}, err
	}

	for _, contentMigrationFilepath := range inputContentMigrations {
		c.logger.Printf("Adding %s to content migrations...", filepath.Base(contentMigrationFilepath))

		contentMigrationContent, err := ioutil.ReadFile(contentMigrationFilepath)
		if err != nil {
			return []byte{}, err
		}

		var contentMigration interface{}
		err = yaml.Unmarshal(contentMigrationContent, &contentMigration)
		if err != nil {
			return []byte{}, err
		}

		contentMigrations.Migrations = append(contentMigrations.Migrations, contentMigration)
	}

	contentMigrationContents, err := yaml.Marshal(contentMigrations)
	if err != nil {
		return []byte{}, err
	}

	versionRegexp := regexp.MustCompile(`\d+.\d+.\d+.[0-9]\$PRERELEASE_VERSION\$`)

	c.logger.Printf("Injecting version %q into content migrations...", version)
	contentMigrationContents = versionRegexp.ReplaceAll(contentMigrationContents, []byte(version))

	return contentMigrationContents, nil
}

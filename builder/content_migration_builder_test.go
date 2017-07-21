package builder_test

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pivotal-cf/kiln/builder"
	"github.com/pivotal-cf/kiln/builder/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContentMigrationBuilder", func() {
	Describe("Build", func() {
		var (
			tempDir                 string
			baseContentMigration    string
			version                 string
			contentMigrations       []string
			logger                  *fakes.Logger
			contentMigrationBuilder builder.ContentMigrationBuilder
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			logger = &fakes.Logger{}
			version = "7.8.9-build.10"

			baseContentMigrationContents := `---
product: my-product
installation_schema_version: "1.6"
to_version: "7.8.9.0$PRERELEASE_VERSION$"
migrations: []`

			baseContentMigration = filepath.Join(tempDir, "base.yml")
			err = ioutil.WriteFile(baseContentMigration, []byte(baseContentMigrationContents), 0644)
			Expect(err).NotTo(HaveOccurred())

			contentMigrationContents := `---
from_version: 1.6.0-build.315
rules:
  - type: update
    selector: "product_version"
    to: "7.8.9.0$PRERELEASE_VERSION$"
`
			contentMigration := filepath.Join(tempDir, "from-1.6.0-build.315.yml")
			err = ioutil.WriteFile(contentMigration, []byte(contentMigrationContents), 0644)
			Expect(err).NotTo(HaveOccurred())
			contentMigrations = []string{contentMigration}

			contentMigrationBuilder = builder.NewContentMigrationBuilder(logger)
		})

		It("creates the content_migrations/migrations.yml file", func() {
			finalContentMigration, err := contentMigrationBuilder.Build(baseContentMigration, version, contentMigrations)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(finalContentMigration)).To(Equal(`product: my-product
installation_schema_version: "1.6"
to_version: 7.8.9-build.10
migrations:
- from_version: 1.6.0-build.315
  rules:
  - selector: product_version
    to: 7.8.9-build.10
    type: update
`))

			Expect(logger.PrintfCall.Receives.LogLines).To(ContainElement("Injecting version \"7.8.9-build.10\" into content migrations..."))
		})

		It("logs each content migration that is added to the file", func() {
			_, err := contentMigrationBuilder.Build(baseContentMigration, version, contentMigrations)
			Expect(err).NotTo(HaveOccurred())

			Expect(logger.PrintfCall.Receives.LogLines).To(ContainElement("Adding from-1.6.0-build.315.yml to content migrations..."))
		})

		Context("failure cases", func() {

			var contentMigrations []string

			Context("when the base content migration cannot be read", func() {
				It("returns an error", func() {
					contentMigrationBuilder = builder.NewContentMigrationBuilder(logger)

					_, err := contentMigrationBuilder.Build("missing-base", version, contentMigrations)
					Expect(err).To(MatchError("open missing-base: no such file or directory"))
				})
			})

			Context("when the contents of the base content migration is invalid", func() {
				It("returns an error", func() {
					baseContentMigrationContents := "SOME INVALID CONTENT"

					baseContentMigration = filepath.Join(tempDir, "base.yml")
					err := ioutil.WriteFile(baseContentMigration, []byte(baseContentMigrationContents), 0644)
					Expect(err).NotTo(HaveOccurred())

					contentMigrationBuilder = builder.NewContentMigrationBuilder(logger)

					_, err = contentMigrationBuilder.Build(baseContentMigration, version, contentMigrations)
					Expect(err.Error()).To(ContainSubstring("unmarshal errors"))
				})
			})

			Context("when a content migration cannot be read", func() {
				It("returns an error", func() {
					contentMigrationBuilder = builder.NewContentMigrationBuilder(logger)

					_, err := contentMigrationBuilder.Build(baseContentMigration, version, []string{"missing-migration"})
					Expect(err).To(MatchError("open missing-migration: no such file or directory"))
				})
			})

			Context("when the contents of a migration is invalid", func() {
				It("returns an error", func() {
					contentMigrationContents := "%%%%%"

					contentMigration := filepath.Join(tempDir, "###from-1.6.1-build.1.yml")
					err := ioutil.WriteFile(contentMigration, []byte(contentMigrationContents), 0644)
					Expect(err).NotTo(HaveOccurred())
					contentMigrations = append(contentMigrations, contentMigration)

					contentMigrationBuilder = builder.NewContentMigrationBuilder(logger)

					_, err = contentMigrationBuilder.Build(baseContentMigration, version, contentMigrations)
					Expect(err.Error()).To(ContainSubstring("yaml: could not find expected directive name"))
				})
			})
		})
	})
})

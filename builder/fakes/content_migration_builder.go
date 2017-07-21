package fakes

type ContentMigrationBuilder struct {
	BuildCall struct {
		CallCount int
		Receives  struct {
			BaseContentMigration string
			ContentMigrations    []string
			Version              string
		}

		Returns struct {
			ContentMigration []byte
			Error            error
		}
	}
}

func (c *ContentMigrationBuilder) Build(baseContentMigration string, version string, contentMigrations []string) ([]byte, error) {
	c.BuildCall.CallCount++
	c.BuildCall.Receives.BaseContentMigration = baseContentMigration
	c.BuildCall.Receives.ContentMigrations = contentMigrations
	c.BuildCall.Receives.Version = version
	return c.BuildCall.Returns.ContentMigration, c.BuildCall.Returns.Error
}

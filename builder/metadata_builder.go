package builder

type MetadataBuilder struct {
	instanceGroupDirectoryReader metadataPartsDirectoryReader
	jobsDirectoryReader          metadataPartsDirectoryReader
	logger                       logger
	metadataReader               metadataReader
}

type BuildInput struct {
	MetadataPath string
	Version      string
}

//go:generate counterfeiter -o ./fakes/metadata_parts_directory_reader.go --fake-name MetadataPartsDirectoryReader . metadataPartsDirectoryReader
type metadataPartsDirectoryReader interface {
	Read(path string) ([]Part, error)
}

//go:generate counterfeiter -o ./fakes/metadata_reader.go --fake-name MetadataReader . metadataReader
type metadataReader interface {
	Read(path, version string) (Metadata, error)
}

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

func NewMetadataBuilder(
	metadataReader metadataReader,
	logger logger,
) MetadataBuilder {
	return MetadataBuilder{
		logger:         logger,
		metadataReader: metadataReader,
	}
}

func (m MetadataBuilder) Build(input BuildInput) (GeneratedMetadata, error) {
	metadata, err := m.metadataReader.Read(input.MetadataPath, input.Version)
	if err != nil {
		return GeneratedMetadata{}, err
	}

	return GeneratedMetadata{
		Metadata: metadata,
	}, nil
}

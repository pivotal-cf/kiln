package baking

//go:generate counterfeiter -o ./fakes/logger.go --fake-name Logger . logger
type logger interface {
	Println(v ...interface{})
}

type StemcellService struct {
	logger logger
	reader partReader
}

func NewStemcellService(logger logger, reader partReader) StemcellService {
	return StemcellService{
		logger: logger,
		reader: reader,
	}
}

func (ss StemcellService) FromTarball(path string) (interface{}, error) {
	if path == "" {
		return nil, nil
	}

	ss.logger.Println("Reading stemcell manifest...")

	stemcell, err := ss.reader.Read(path)
	if err != nil {
		return nil, err
	}

	return stemcell.Metadata, nil
}

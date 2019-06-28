package fakes

type VersionsService struct {
	CallCount int
	Receives  struct {
		StemcellOS string
	}
	Returns struct {
		Versions []string
		Err      error
	}
}

func (mock *VersionsService) Versions(stemcellOS string) ([]string, error) {
	mock.CallCount++
	mock.Receives.StemcellOS = stemcellOS
	return mock.Returns.Versions, mock.Returns.Err
}

package fakes

type VersionsService struct {
	VersionsCall struct {
		CallCount int
		Receives  struct {
			StemcellOS string
		}
		Returns struct {
			Versions []string
			Err      error
		}
	}
	SetTokenCall struct {
		CallCount int
		Receives  struct {
			Token string
		}
	}
}

func (mock *VersionsService) Versions(stemcellOS string) ([]string, error) {
	mock.VersionsCall.CallCount++
	mock.VersionsCall.Receives.StemcellOS = stemcellOS
	return mock.VersionsCall.Returns.Versions, mock.VersionsCall.Returns.Err
}

func (mock *VersionsService) SetToken(token string) {
	mock.SetTokenCall.CallCount++
	mock.SetTokenCall.Receives.Token = token
}

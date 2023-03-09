package internal

import (
	"os"
)

//counterfeiter:generate ./fakes/om_runner.go --fake-name OMRunner . OMRunnerI
type OMRunnerI interface {
	ResetAndConfigure(productName string, productVersion string, configJSON string) error
	GetManifest(productGUID string) (map[string]interface{}, error)
	FindStagedProduct(productName string) (StagedProduct, error)
}

//counterfeiter:generate ./fakes/ops_manifest_runner.go --fake-name OpsManifestRunner . OpsManifestRunnerI
type OpsManifestRunnerI interface {
	GetManifest(productProperties, metadataFilePath string) (map[string]interface{}, error)
}

//counterfeiter:generate ./fakes/file_io.go --fake-name FileIO . FileIO
type FileIO interface {
	TempFile(string, string) (*os.File, error)
	Remove(string) error
}

var RealIO = fileIO{}

type fileIO struct{}

func (f fileIO) TempFile(a, b string) (*os.File, error) {
	return os.CreateTemp(a, b)
}

func (f fileIO) Remove(a string) error {
	return os.Remove(a)
}

//counterfeiter:generate ./fakes/command_runner.go --fake-name CommandRunner . CommandRunner
type CommandRunner interface {
	Run(string, ...string) (string, string, error)
}

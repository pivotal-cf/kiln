package internal

import (
	"io/ioutil"
	"os"
)

//go:generate counterfeiter -o ./fakes/om_runner.go --fake-name OMRunner . OMRunnerI
type OMRunnerI interface {
	ResetAndConfigure(productName string, productVersion string, configJSON string) error
	GetManifest(productGUID string) (map[string]interface{}, error)
	FindStagedProduct(productName string) (StagedProduct, error)
}

//go:generate counterfeiter -o ./fakes/ops_manifest_runner.go --fake-name OpsManifestRunner . OpsManifestRunnerI
type OpsManifestRunnerI interface {
	GetManifest(productProperties, metadataFilePath string) (map[string]interface{}, error)
}

//go:generate counterfeiter -o ./fakes/file_io.go --fake-name FileIO . FileIO
type FileIO interface {
	TempFile(string, string) (*os.File, error)
	Remove(string) error
}

var RealIO = fileIO{}

type fileIO struct{}

func (f fileIO) TempFile(a, b string) (*os.File, error) {
	return ioutil.TempFile(a, b)
}

func (f fileIO) Remove(a string) error {
	return os.Remove(a)
}

//go:generate counterfeiter -o ./fakes/command_runner.go --fake-name CommandRunner . CommandRunner
type CommandRunner interface {
	Run(string, ...string) (string, string, error)
}

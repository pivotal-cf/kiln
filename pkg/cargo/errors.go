package cargo

import (
	"fmt"
)

const (
	ErrStemcellOSInfoMustBeValid = "stemcell os information is missing or invalid"
)

func errorSpecNotFound(name string) error {
	return fmt.Errorf("failed to find release with name %q", name)
}

type ConfigFileError struct {
	HumanReadableConfigFileName string
	err                         error
}

func (err ConfigFileError) Unwrap() error {
	return err.err
}

func (err ConfigFileError) Error() string {
	return fmt.Sprintf("encountered a configuration file error with %s: %s", err.HumanReadableConfigFileName, err.err.Error())
}

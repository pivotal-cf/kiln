package internal

import (
	"bytes"
	"os/exec"
)

type EnvironmentSharingCommandRunner struct {
	env []string
}

func NewEnvironmentSharingCommandRunner(env []string) *EnvironmentSharingCommandRunner {
	return &EnvironmentSharingCommandRunner{
		env: env,
	}
}

func (e *EnvironmentSharingCommandRunner) Run(name string, args ...string) (string, string, error) {
	var errBuffer bytes.Buffer

	cmd := exec.Command(name, args...)
	cmd.Env = e.env
	cmd.Stderr = &errBuffer
	output, err := cmd.Output()

	return string(output), errBuffer.String(), err
}

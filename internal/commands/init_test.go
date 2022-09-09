package commands_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/go-git/go-billy/v5"
	"github.com/pivotal-cf/jhanda"
	"gopkg.in/yaml.v3"
)

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "commands")
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o ./fakes/command.go --fake-name Command . command
type command interface {
	jhanda.Command
}

var _ command

func fsWriteYAML(fs billy.Basic, path string, data interface{}) error {
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	f, err := fs.Create(path)
	if err != nil {
		return err
	}
	defer closeAndIgnoreError(f)

	_, err = f.Write(buf)
	return err
}

func fsReadYAML(fs billy.Basic, path string, data interface{}) error {
	f, err := fs.Open(path)
	if err != nil {
		return nil
	}
	defer closeAndIgnoreError(f)

	buf, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(buf, data)
	if err != nil {
		return err
	}

	return err
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

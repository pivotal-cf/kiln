package commands_test

import (
	"io/ioutil"
	"os"

	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/jhanda"
	"gopkg.in/src-d/go-billy.v4"
	"gopkg.in/yaml.v2"
)

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "commands")
}

//go:generate counterfeiter -o ./fakes/command.go --fake-name Command . command
type command interface {
	jhanda.Command
}

var _ command

func writeYAML(path string, data interface{}) error {
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Write(buf)
	return err
}

func readYAML(path string, data interface{}) error {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return nil
	}

	err = yaml.Unmarshal(buf, data)
	if err != nil {
		return err
	}

	return err
}

func fsWriteYAML(fs billy.Basic, path string, data interface{}) error {
	buf, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	f, err := fs.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	_, err = f.Write(buf)
	return err
}

func fsReadYAML(fs billy.Basic, path string, data interface{}) error {
	f, err := fs.Open(path)
	if err != nil {
		return nil
	}
	defer func() {
		_ = f.Close()
	}()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(buf, data)
	if err != nil {
		return err
	}

	return err
}

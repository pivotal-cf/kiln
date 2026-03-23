package models

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type CarvelLockfile struct {
	Releases []CarvelReleaseLock `yaml:"releases"`
}

type CarvelReleaseLock struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	RemotePath string `yaml:"remote_path"`
	SHA256     string `yaml:"sha256"`
}

func ReadCarvelLockfile(path string) (CarvelLockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CarvelLockfile{}, fmt.Errorf("failed to read lockfile: %w", err)
	}
	var lf CarvelLockfile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return CarvelLockfile{}, fmt.Errorf("failed to parse lockfile: %w", err)
	}
	return lf, nil
}

func (lf CarvelLockfile) WriteFile(path string) error {
	data, err := yaml.Marshal(&lf)
	if err != nil {
		return fmt.Errorf("failed to marshal lockfile: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

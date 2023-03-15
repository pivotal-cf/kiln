package planitest

import (
	"fmt"

	"github.com/cppforlife/go-patch/patch"
	yaml "gopkg.in/yaml.v2"
)

type Manifest string

func (m *Manifest) FindInstanceGroupJob(instanceGroup, job string) (Manifest, error) {
	path := fmt.Sprintf("/instance_groups/name=%s/jobs/name=%s", instanceGroup, job)

	result, err := m.interpolate(path)
	if err != nil {
		return "", err
	}

	content, err := yaml.Marshal(result)
	if err != nil {
		return "", err // should never happen
	}
	return Manifest(string(content)), nil
}

func (m Manifest) Property(path string) (interface{}, error) {
	return m.interpolate(fmt.Sprintf("/properties/%s", path))
}

func (m Manifest) Path(path string) (interface{}, error) {
	return m.interpolate(path)
}

func (m Manifest) interpolate(path string) (interface{}, error) {
	var content interface{}
	err := yaml.Unmarshal([]byte(m), &content)
	if err != nil {
		return "", fmt.Errorf("failed to parse manifest: %s", err)
	}

	res, err := patch.FindOp{Path: patch.MustNewPointerFromString(path)}.Apply(content)
	if err != nil {
		return "", fmt.Errorf("failed to find value at path '%s': %s", path, m)
	}

	return res, nil
}

func (m Manifest) String() string {
	return string(m)
}

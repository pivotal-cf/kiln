package commands

import yaml "gopkg.in/yaml.v2"

func SetYamlMarshal(f func(interface{}) ([]byte, error)) {
	yamlMarshal = f
}

func ResetYamlMarshal() {
	yamlMarshal = yaml.Marshal
}

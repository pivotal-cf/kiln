package internal

import (
	"fmt"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type OpsManifestRunner struct {
	cmdRunner      CommandRunner
	FileIO         FileIO
	additionalArgs []string
}

func NewOpsManifestRunner(cmdRunner CommandRunner, fileIO FileIO, additionalArgs ...string) OpsManifestRunner {
	return OpsManifestRunner{
		cmdRunner:      cmdRunner,
		FileIO:         fileIO,
		additionalArgs: additionalArgs,
	}
}

func (o OpsManifestRunner) GetManifest(productProperties, metadataFilePath string) (map[string]interface{}, error) {
	fmt.Printf("%+v\n product props ^ \n", productProperties)
	fmt.Println("Get manifest")
	configFile, err := o.FileIO.TempFile("", "")
	configFileYML := fmt.Sprintf("%s.yml", configFile.Name())
	os.Rename(configFile.Name(), configFileYML)
	fmt.Println("configFile: ", configFileYML)

	if err != nil {
		fmt.Println("31")
		return nil, err //not tested
	}

	_, err = configFile.WriteString(productProperties)
	if err != nil {
		fmt.Println("37")
		return nil, err //not tested
	}

	fmt.Println("metadata path ", metadataFilePath)
	args := []string{"--config-file", configFileYML, "--metadata-path", metadataFilePath}
	args = append(args, o.additionalArgs...)

	response, errOutput, err := o.cmdRunner.Run("ops-manifest", args...)

	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve manifest: %s: %s", err, errOutput)
	}

	var manifest map[string]interface{}
	err = yaml.Unmarshal([]byte(response), &manifest)
	if err != nil {
		return nil, fmt.Errorf("Unable to unmarshal yaml: %s", err)
	}

	return manifest, nil
}

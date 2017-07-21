package builder

import (
	"io/ioutil"
	"regexp"

	"gopkg.in/yaml.v2"
)

type HandcraftReader struct {
	filesystem filesystem
	logger     logger
}

type Handcraft map[string]interface{}

func NewHandcraftReader(filesystem filesystem, logger logger) HandcraftReader {
	return HandcraftReader{
		filesystem: filesystem,
		logger:     logger,
	}
}

func (h HandcraftReader) Read(path, version string) (Handcraft, error) {
	file, err := h.filesystem.Open(path)
	if err != nil {
		return Handcraft{}, err
	}

	contents, err := ioutil.ReadAll(file)
	if err != nil {
		return Handcraft{}, err
	}

	versionRegexp := regexp.MustCompile(`\d+.\d+.\d+.[0-9]\$PRERELEASE_VERSION\$`)

	h.logger.Printf("Injecting version %q into handcraft...", version)
	contents = versionRegexp.ReplaceAll(contents, []byte(version))

	var handcraft Handcraft
	err = yaml.Unmarshal(contents, &handcraft)
	if err != nil {
		return Handcraft{}, err
	}

	return handcraft, nil
}

package builder

import (
	"encoding/base64"
	"io/ioutil"
)

type IconEncoder struct {
	filesystem filesystem
}

func NewIconEncoder(filesystem filesystem) IconEncoder {
	return IconEncoder{
		filesystem: filesystem,
	}
}

func (i IconEncoder) Encode(path string) (string, error) {
	file, err := i.filesystem.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

package baking

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

type Checksummer struct {
	logger logger
}

func NewChecksummer(logger logger) Checksummer {
	return Checksummer{logger: logger}
}

func (c Checksummer) Sum(path string) error {
	c.logger.Println(fmt.Sprintf("Calculating SHA256 checksum of %s...", path))

	hash := sha256.New()

	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(hash, file)
	if err != nil {
		return err
	}

	hexsum := fmt.Sprintf("%x", hash.Sum(nil))

	err = ioutil.WriteFile(fmt.Sprintf("%s.sha256", path), []byte(hexsum), 0644)
	if err != nil {
		return err
	}

	c.logger.Println(fmt.Sprintf("SHA256 checksum: %s", hexsum))

	return nil
}

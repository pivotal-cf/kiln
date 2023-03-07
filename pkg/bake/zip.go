package bake

import (
	"fmt"
	"io"
	"io/fs"

	"gopkg.in/yaml.v3"
)

func Tile(out io.Writer, tileDirectory fs.FS, o Options) error {
	tc, metadataNode, err := metadataSetup(tileDirectory, o)
	if err != nil {
		return err
	}
	releases, err := newReleasesFromDirectories(tileDirectory, o.Releases)
	if err != nil {
		return err
	}
	tc.templateFunctions["releases"] = releaseWithName(releases)
	result, err := interpolate(tc, newPart(tc.options.Metadata, metadataNode))
	if err != nil {
		return err
	}
	resultBuffer, err := yaml.Marshal(result)
	if err != nil {
		return err
	}
	_, err = out.Write(resultBuffer)
	if err != nil {
		return err
	}
	return nil
}

func releaseWithName(releases []release) func(name string) (string, error) {
	return func(name string) (string, error) {
		for _, rel := range releases {
			if rel.Name != name {
				continue
			}
			return encodeJSONString(rel, nil)
		}
		return "", fmt.Errorf("failed to find release in directory with name %q", name)
	}
}

package tile

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	yamlConverter "github.com/ghodss/yaml"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/proofing"
)

const (
	MetadataTileFilePath = "metadata/metadata.yml"

	MetadataDefaultSourceFilePath              = "base.yml"
	VersionDefaultSourceFilePath               = "version"
	FormsDirectoryDefaultFilePath              = "forms"
	InstanceGroupsDirectoryPathsFilePath       = "instance_groups"
	JobsDirectoryDefaultFilePath               = "jobs"
	PropertiesDirectoryDefaultPathsFilePath    = "properties"
	RuntimeConfigurationsDefaultDirectoryPaths = "runtime_configs"
	VariablesDefaultDirectoryFilePath          = "variables"
	IconDefaultSourceFilePath                  = "icon.png"

	MetadataGitSHAVariable = "metadata-git-sha"
)

type BakeConfiguration struct {
	MetadataTemplateFilePath string
	FormsDirectoryPaths,
	PropertiesDirectoryPaths,
	InstanceGroupsDirectoryPaths,
	JobsDirectoryPaths,
	VariablesFilePaths,
	RuntimeConfigurationsDirectoryPaths []string
	TileVersion  string
	IconFilePath string
	Releases     []proofing.Release
	Variables    map[string]string
}

// CalculateSourceChecksum sets the MetadataGitSHAVariable variable.
func (bakeConfiguration *BakeConfiguration) CalculateSourceChecksum(tileDirectory string) error {
	bakeConfiguration.ensureVariables()
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("could not calculate %q: %w", MetadataGitSHAVariable, err)
	}
	var statusOutput bytes.Buffer
	gitStatus := exec.Command("git", "status", "--porcelain", "--untracked-files")
	gitStatus.Dir = tileDirectory
	gitStatus.Stdout = &statusOutput
	gitStatus.Stdout = &statusOutput
	err := gitStatus.Run()
	if err != nil {
		return fmt.Errorf("could not calculate %q: failed to run `%s %s` %s: %w", MetadataGitSHAVariable, gitStatus.Path, strings.Join(gitStatus.Args, " "), strings.TrimSpace(statusOutput.String()), err)
	}
	if strings.TrimSpace(statusOutput.String()) != "" {
		return fmt.Errorf("could not calculate %q: git working directory has un-commited changes", MetadataGitSHAVariable)
	}
	var out bytes.Buffer
	gitRevParseHead := exec.Command("git", "rev-parse", "HEAD")
	gitRevParseHead.Dir = tileDirectory
	gitRevParseHead.Stdout = &out
	err = gitRevParseHead.Run()
	if err != nil {
		return fmt.Errorf("could not calculate %q: failed to get HEAD revision hash %s: %w", MetadataGitSHAVariable, strings.TrimSpace(out.String()), err)
	}
	bakeConfiguration.Variables[MetadataGitSHAVariable] = strings.TrimSpace(out.String())
	return nil
}

func (bakeConfiguration *BakeConfiguration) validate() error {
	if bakeConfiguration.MetadataTemplateFilePath == "" {
		return fmt.Errorf("missing metadata file expecting %s", MetadataDefaultSourceFilePath)
	}
	if bakeConfiguration.IconFilePath == "" {
		return fmt.Errorf("missing icon file %s", IconDefaultSourceFilePath)
	}
	return nil

}

func (bakeConfiguration *BakeConfiguration) setDefaults(tileDirectory fs.FS) {
	if bakeConfiguration.MetadataTemplateFilePath == "" {
		_, err := fs.Stat(tileDirectory, MetadataDefaultSourceFilePath)
		if err == nil {
			bakeConfiguration.MetadataTemplateFilePath = MetadataDefaultSourceFilePath
		}
	}
	if bakeConfiguration.TileVersion == "" {
		versionContents, err := fs.ReadFile(tileDirectory, VersionDefaultSourceFilePath)
		if err == nil {
			bakeConfiguration.TileVersion = string(versionContents)
		}
	}
	if bakeConfiguration.IconFilePath == "" {
		_, err := fs.Stat(tileDirectory, IconDefaultSourceFilePath)
		if err == nil {
			bakeConfiguration.IconFilePath = IconDefaultSourceFilePath
		}
	}
	if len(bakeConfiguration.PropertiesDirectoryPaths) == 0 {
		info, err := fs.Stat(tileDirectory, PropertiesDirectoryDefaultPathsFilePath)
		if err == nil && info.IsDir() {
			bakeConfiguration.PropertiesDirectoryPaths = []string{PropertiesDirectoryDefaultPathsFilePath}
		}
	}
	if len(bakeConfiguration.InstanceGroupsDirectoryPaths) == 0 {
		info, err := fs.Stat(tileDirectory, InstanceGroupsDirectoryPathsFilePath)
		if err == nil && info.IsDir() {
			bakeConfiguration.InstanceGroupsDirectoryPaths = []string{InstanceGroupsDirectoryPathsFilePath}
		}
	}
	if len(bakeConfiguration.JobsDirectoryPaths) == 0 {
		info, err := fs.Stat(tileDirectory, JobsDirectoryDefaultFilePath)
		if err == nil && info.IsDir() {
			bakeConfiguration.JobsDirectoryPaths = []string{JobsDirectoryDefaultFilePath}
		}
	}
	if len(bakeConfiguration.VariablesFilePaths) == 0 {
		entrees, _ := fs.ReadDir(tileDirectory, VariablesDefaultDirectoryFilePath)
		if len(entrees) == 1 {
			bakeConfiguration.VariablesFilePaths = []string{filepath.Join(VariablesDefaultDirectoryFilePath, entrees[0].Name())}
		}
	}
	if len(bakeConfiguration.RuntimeConfigurationsDirectoryPaths) == 0 {
		entrees, _ := fs.ReadDir(tileDirectory, RuntimeConfigurationsDefaultDirectoryPaths)
		if len(entrees) == 1 {
			bakeConfiguration.RuntimeConfigurationsDirectoryPaths = []string{RuntimeConfigurationsDefaultDirectoryPaths}
		}
	}
	if len(bakeConfiguration.FormsDirectoryPaths) == 0 {
		info, err := fs.Stat(tileDirectory, FormsDirectoryDefaultFilePath)
		if err == nil && info.IsDir() {
			bakeConfiguration.FormsDirectoryPaths = []string{FormsDirectoryDefaultFilePath}
		}
	}
	bakeConfiguration.ensureVariables()
}

func (bakeConfiguration *BakeConfiguration) ensureVariables() {
	if bakeConfiguration.Variables == nil {
		bakeConfiguration.Variables = make(map[string]string)
	}
}

func errVariableNotFound(name string) error {
	return fmt.Errorf("variable with name %q not found", name)
}

func (bakeConfiguration *BakeConfiguration) Variable(name string) (string, error) {
	if bakeConfiguration == nil || bakeConfiguration.Variables == nil {
		return "", errVariableNotFound(name)
	}
	value, ok := bakeConfiguration.Variables[name]
	if !ok {
		return "", errVariableNotFound(name)
	}
	return value, nil
}

func (bakeConfiguration *BakeConfiguration) release(name string) (string, error) {
	index := slices.IndexFunc(bakeConfiguration.Releases, func(release proofing.Release) bool {
		return release.Name == name
	})
	if index < 0 {
		return "", fmt.Errorf("release %q not found", name)
	}
	releaseJSON, err := json.Marshal(bakeConfiguration.Releases[index])
	return string(releaseJSON), err
}

const PanicBakeImplementationIncomplete = "bake implementation is incomplete"

// Bake is not yet fully implemented.
// THIS FUNCTION WILL PANIC UNTIL IT IS COMPLETE.
func Bake(w io.Writer, tileDirectory fs.FS, configuration *BakeConfiguration, logger *log.Logger) error {
	w, tileDirectory, configuration, logger = bakePrecondition(w, tileDirectory, configuration, logger)
	if err := configuration.validate(); err != nil {
		return err
	}
	tile := zip.NewWriter(w)
	if err := writeMetadataFile(tile, tileDirectory, configuration, logger); err != nil {
		return err
	}
	err := tile.Close()
	if err != nil {
		return err
	}
	panic(PanicBakeImplementationIncomplete)
}

func bakePrecondition(w io.Writer, tileDirectory fs.FS, configuration *BakeConfiguration, logger *log.Logger) (io.Writer, fs.FS, *BakeConfiguration, *log.Logger) {
	if w == nil {
		w = io.Discard
	}
	//if tileDirectory == nil {
	//	tileDirectory = os.DirFS(".")
	//}
	if configuration == nil {
		configuration = new(BakeConfiguration)
	}
	configuration.setDefaults(tileDirectory)
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return w, tileDirectory, configuration, logger
}

func writeMetadataFile(z *zip.Writer, d fs.FS, sc *BakeConfiguration, logger *log.Logger) error {
	f, err := z.Create(MetadataTileFilePath)
	if err != nil {
		return err
	}
	return Metadata(f, d, sc, logger)
}

// Metadata is not fully implemented.
func Metadata(w io.Writer, tileDirectory fs.FS, configuration *BakeConfiguration, logger *log.Logger) error {
	w, tileDirectory, configuration, logger = bakePrecondition(w, tileDirectory, configuration, logger)
	if err := configuration.validate(); err != nil {
		return err
	}
	if err := loadVariablesFromFiles(configuration, tileDirectory); err != nil {
		return err
	}
	interpolatedMetadata, err := interpolate(tileDirectory, configuration.MetadataTemplateFilePath, configuration)
	if err != nil {
		return err
	}
	_, err = w.Write(interpolatedMetadata)
	return err
}

func loadVariablesFromFiles(configuration *BakeConfiguration, tileDirectory fs.FS) error {
	for _, variablesFile := range configuration.VariablesFilePaths {
		buf, err := fs.ReadFile(tileDirectory, variablesFile)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(buf, configuration.Variables); err != nil {
			return err
		}
	}
	return nil
}

func readAndEncodeIcon(tileDirectory fs.FS, bakeConfiguration *BakeConfiguration) (string, error) {
	buf, err := fs.ReadFile(tileDirectory, bakeConfiguration.IconFilePath)
	if err != nil {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func interpolate(tileDirectory fs.FS, filePath string, bakeConfiguration *BakeConfiguration) ([]byte, error) {
	t, err := template.New(path.Base(filePath)).
		Delims("$(", ")").
		Funcs(templateFunctions(tileDirectory, bakeConfiguration)).
		Option("missingkey=error").
		ParseFS(tileDirectory, filePath)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	err = t.Execute(&out, bakeConfiguration.Variables)
	if err != nil {
		return nil, err
	}
	return out.Bytes(), err
}

func templateFunctions(tileDirectory fs.FS, bakeConfiguration *BakeConfiguration) template.FuncMap {
	return template.FuncMap{
		"version": func() string { return bakeConfiguration.TileVersion },
		"property": func(name string) (string, error) {
			return resolveMetadata(tileDirectory, name, bakeConfiguration.PropertiesDirectoryPaths, bakeConfiguration)
		},
		"icon": func() (string, error) {
			return readAndEncodeIcon(tileDirectory, bakeConfiguration)
		},
		"instance_group": func(name string) (string, error) {
			return resolveMetadata(tileDirectory, name, bakeConfiguration.InstanceGroupsDirectoryPaths, bakeConfiguration)
		},
		"job": func(name string) (string, error) {
			return resolveMetadata(tileDirectory, name, bakeConfiguration.JobsDirectoryPaths, bakeConfiguration)
		},
		"variable": bakeConfiguration.Variable,
		"release":  bakeConfiguration.release,
		"form": func(name string) (string, error) {
			return resolveMetadata(tileDirectory, name, bakeConfiguration.FormsDirectoryPaths, bakeConfiguration)
		},
		"runtime_config": func(name string) (string, error) {
			return resolveMetadata(tileDirectory, name, bakeConfiguration.RuntimeConfigurationsDirectoryPaths, bakeConfiguration)
		},
	}
}

func resolveMetadata(tileDirectory fs.FS, name string, directories []string, bakeConfiguration *BakeConfiguration) (string, error) {
	for _, directoryName := range directories {
		foundMetadata := false
		var out bytes.Buffer
		walkError := fs.WalkDir(tileDirectory, directoryName, func(filePath string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			switch path.Ext(filePath) {
			case ".yml", ".yaml":
				buf, err := interpolate(tileDirectory, filePath, bakeConfiguration)
				if err != nil {
					return err
				}
				found, err := namedMapping(&out, name, buf)
				if err != nil {
					return err
				}
				if found {
					foundMetadata = true
					return fs.SkipAll
				}
			}
			return nil
		})
		if walkError != nil {
			return "", walkError
		}
		if foundMetadata {
			return out.String(), nil
		}
	}
	return "", fmt.Errorf("failed to find metadata with name %q", name)
}

func namedMapping(out io.Writer, name string, buf []byte) (bool, error) {
	var node yaml.Node
	err := yaml.Unmarshal(buf, &node)
	if err != nil {
		return false, err
	}

	type metadata struct {
		Name  string `yaml:"name"`
		Alias string `yaml:"alias"`
	}

	switch node.Content[0].Kind {
	case yaml.MappingNode:
		var data metadata
		_ = node.Content[0].Decode(&data)
		switch name {
		case data.Alias, data.Name:
			if err := encodeNode(out, node.Content[0]); err != nil {
				return false, err
			}
			return true, nil
		}
	case yaml.SequenceNode:
		var list []metadata
		_ = node.Content[0].Decode(&list)
		for i, elem := range list {
			switch name {
			case elem.Alias, elem.Name:
				if err := encodeNode(out, node.Content[0].Content[i]); err != nil {
					return false, err
				}
				return true, nil
			}
		}
	}
	return false, nil
}

func encodeNode(out io.Writer, node *yaml.Node) error {
	metadataYAML, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	metadataJSON, err := yamlConverter.YAMLToJSON(metadataYAML)
	if err != nil {
		return err
	}
	_, err = out.Write(metadataJSON)
	if err != nil {
		return err
	}
	return nil
}

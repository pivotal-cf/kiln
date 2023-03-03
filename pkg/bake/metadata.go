package bake

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/crhntr/yamlutil/yamlconv"
	"github.com/crhntr/yamlutil/yamlnode"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

const (
	interpolateDelimiterStart           = "$("
	interpolateDelimiterEnd             = ")"
	interpolateTemplateMissingKeyOption = "missingkey=error"

	TileNameVariable = "tile_name"

	defaultVariablesDirectory = "variables"

	allYAMLNodeKinds = yaml.DocumentNode | yaml.SequenceNode | yaml.MappingNode | yaml.ScalarNode | yaml.AliasNode

	buildVersionVariableName = "build-version"
)

type Variables map[string]any

type Options struct {
	// Metadata specifies a path to the root metadata template.
	Metadata string `default_path:"base.yml"`

	// Icon specifies the filepath to an image to base64 encode into the tile.
	Icon string `default_path:"icon.png"`

	Kilnfile string `default_path:"Kilnfile"`

	// Version specifies a filepath to a file containing a version.
	// If VersionValue is set, this field is ignored.
	VersionFile string `default_path:"version"`

	// VersionValue specifies the string for the tile version.
	VersionValue string

	InstanceGroups []string `default_path:"instance_groups"`
	Forms          []string `default_path:"forms"`
	Jobs           []string `default_path:"jobs"`
	RuntimeConfigs []string `default_path:"runtime_configs"`
	Properties     []string `default_path:"properties"`

	// VariablesFiles may provide additional variables files.
	// If a file "variables/${TILE_NAME}.yml" (TileName) exists, it will be added even if not provided in VariablesFiles.
	VariablesFiles []string

	TileName string

	// Variables contains values that may be used in conditional expressions for templating
	// or as results of the "variable" template function.
	Variables Variables
}

func setDefaultOptions(o *Options, tileDirectory fs.FS) {
	v := reflect.ValueOf(o).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if !v.Type().Field(i).IsExported() || !f.IsZero() {
			continue
		}
		defaultFilePath := v.Type().Field(i).Tag.Get("default_path")
		_, err := fs.Stat(tileDirectory, defaultFilePath)
		if err != nil {
			continue
		}
		switch f.Kind() {
		case reflect.Slice:
			f.Set(reflect.Append(f, reflect.ValueOf(defaultFilePath)))
		case reflect.String:
			f.SetString(defaultFilePath)
		}
	}
	if o.Variables == nil {
		o.Variables = make(Variables)
	}
}

func (o Options) metadata(tileDirectory fs.FS) (*yaml.Node, error) {
	buf, err := fs.ReadFile(tileDirectory, o.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}
	buf, err = preProcess(o.TileName, o.Metadata, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to pre-process metadata file: %w", err)
	}
	var n yaml.Node
	return &n, yaml.Unmarshal(buf, &n)
}

func (o Options) kilnfileLock(tileDirectory fs.FS) (cargo.KilnfileLock, error) {
	if o.Kilnfile == "" {
		return cargo.KilnfileLock{}, fmt.Errorf("either the Kilnfile is missing or the path was not configured properly")
	}
	buf, err := fs.ReadFile(tileDirectory, o.Kilnfile+".lock")
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to read Kilnfile.lock: %w", err)
	}
	var lock cargo.KilnfileLock
	err = yaml.Unmarshal(buf, &lock)
	if err != nil {
		return cargo.KilnfileLock{}, fmt.Errorf("failed to parse Kilnfile.lock: %w", err)
	}
	return lock, nil
}

func loadVariables(dir fs.FS, tileName string, vars Variables) error {
	if tileName == "" {
		return nil
	}
	for _, tryName := range slices.Compact(crossProduct([]string{tileName, strings.ToLower(tileName), strings.ToUpper(tileName)}, []string{".yml", ".yaml"})) {
		p := path.Join(defaultVariablesDirectory, tryName)
		_, err := fs.Stat(dir, p)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			continue
		}
		return loadVariablesFile(dir, p, vars)
	}
	return nil
}

func loadVariablesFile(dir fs.FS, p string, vars Variables) error {
	buf, err := fs.ReadFile(dir, p)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(buf, &vars)
}

func Metadata(out io.Writer, tileDirectory fs.FS, o Options) error {
	setDefaultOptions(&o, tileDirectory)
	if err := loadVariables(tileDirectory, o.TileName, o.Variables); err != nil {
		return fmt.Errorf("failed to load variables: %w", err)
	}
	metadataNode, err := o.metadata(tileDirectory)
	if err != nil {
		return fmt.Errorf("failed to open metadata: %w", err)
	}
	lock, err := o.kilnfileLock(tileDirectory)
	if err != nil {
		return fmt.Errorf("failed to open kilnfile: %w", err)
	}
	tc := &templateContext{
		tileName:      o.TileName,
		tileDirectory: tileDirectory,
		options:       o,
		kilnfileLock:  lock,
	}
	if _, found := o.Variables[buildVersionVariableName]; !found {
		v, err := tc.version()
		if err == nil {
			o.Variables[buildVersionVariableName] = v
		}
	}
	tc.templateFunctions = template.FuncMap{
		"icon":            tc.iconTemplateFunction,
		"version":         tc.version,
		"release":         tc.releaseFromKilnfileLock,
		"stemcell":        tc.stemcellFromKilnfileLock,
		"variable":        tc.variable,
		"regexReplaceAll": regexReplaceAll,
	}
	for _, row := range []struct {
		tmplFnName string
		configDirs []string
		nameParts  *namedParts
	}{
		{tmplFnName: "form", configDirs: o.Forms, nameParts: &tc.forms},
		{tmplFnName: "instance_group", configDirs: o.InstanceGroups, nameParts: &tc.instanceGroups},
		{tmplFnName: "job", configDirs: o.Jobs, nameParts: &tc.jobs},
		{tmplFnName: "runtime_config", configDirs: o.RuntimeConfigs, nameParts: &tc.runtimeConfigs},
		{tmplFnName: "property", configDirs: o.Properties, nameParts: &tc.properties},
	} {
		parts := make(map[string]part)
		if len(row.configDirs) == 0 {
			*row.nameParts = parts
			continue
		}
		for _, dir := range row.configDirs {
			err := parseAndPreprocessMetadataPart(tc.tileDirectory, tc.tileName, dir, parts)
			if err != nil {
				return fmt.Errorf("failed to open %s: %w", row.configDirs, err)
			}
		}
		tc.templateFunctions[row.tmplFnName] = createMetadataFunc(parts, tc)
		*row.nameParts = parts
	}
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

func interpolate(tc *templateContext, p part) (*yaml.Node, error) {
	md, err := yaml.Marshal(p.node)
	if err != nil {
		return nil, err
	}
	finalTemplate, err := template.New(p.fileName).
		Funcs(tc.templateFunctions).
		Delims(interpolateDelimiterStart, interpolateDelimiterEnd).
		Option(interpolateTemplateMissingKeyOption).
		Parse(string(md))
	if err != nil {
		return nil, fmt.Errorf("failed interpolation: failed parsing: %w", err)
	}
	var buf bytes.Buffer
	err = finalTemplate.Execute(&buf, tc.options.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed interpolation: failed executing: %w", err)
	}
	var interpolated yaml.Node
	if err := yaml.Unmarshal(buf.Bytes(), &interpolated); err != nil {
		return nil, err
	}
	sortKeysAlphabetically(&interpolated)
	removeSetNullNode(&interpolated)
	removeUnderscoreFromIntegers(&interpolated)
	return &interpolated, nil
}

func preProcess(tileName, name string, metadata []byte) ([]byte, error) {
	initialInterpolation, err := template.New(name).
		Funcs(template.FuncMap{"tile": func() (string, error) {
			if tileName == "" {
				return "", fmt.Errorf("variable %s not set", TileNameVariable)
			}
			return tileName, nil
		}}).
		Option("missingkey=error").
		Parse(string(metadata))
	if err != nil {
		return nil, err
	}
	var intermediateTemplate bytes.Buffer
	if err := initialInterpolation.Execute(&intermediateTemplate, struct{}{}); err != nil {
		return nil, err
	}
	return intermediateTemplate.Bytes(), nil
}

type namedParts map[string]part

func (np *namedParts) add(fileName string, node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		return np.add(fileName, node.Content[0])
	case yaml.SequenceNode:
		var nms []namedMetadata
		if err := node.Decode(&nms); err != nil {
			return err
		}
		for i, nm := range nms {
			(*np)[nm.key()] = newPart(fileName, node.Content[i])
		}
	case yaml.MappingNode:
		var nm namedMetadata
		if err := node.Decode(&nm); err != nil {
			return err
		}
		(*np)[nm.key()] = newPart(fileName, node)
	}
	return nil
}

type namedMetadata struct {
	Name  string `yaml:"name"`
	Alias string `yaml:"alias"`
}

func (nm namedMetadata) key() string {
	if nm.Alias != "" {
		return nm.Alias
	}
	return nm.Name
}

type part struct {
	fileName string
	node     *yaml.Node
}

func newPart(fileName string, node *yaml.Node) part {
	_ = yamlnode.Walk(node, func(n *yaml.Node) error {
		n.FootComment = ""
		n.HeadComment = ""
		n.LineComment = ""
		return nil
	}, allYAMLNodeKinds)
	removeAlias(node)
	return part{
		fileName: fileName,
		node:     node,
	}
}

func removeAlias(node *yaml.Node) {
	_ = yamlnode.Walk(node, func(node *yaml.Node) error {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			switch key {
			case "alias":
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
			}
		}
		return nil
	}, yaml.MappingNode)
}

func removeSetNullNode(node *yaml.Node) {
	_ = yamlnode.Walk(node, func(node *yaml.Node) error {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			switch key {
			case "_hack":
				node.Content[i+1] = &yaml.Node{
					Kind:  yaml.ScalarNode,
					Value: "null",
					Tag:   "!!null",
				}
			}
		}
		return nil
	}, yaml.MappingNode)
}

type templateContext struct {
	tileName string

	options       Options
	tileDirectory fs.FS

	kilnfileLock cargo.KilnfileLock

	instanceGroups, jobs, forms, properties, runtimeConfigs namedParts

	templateFunctions template.FuncMap
}

func (tc *templateContext) iconTemplateFunction() (string, error) {
	iconPNG, err := fs.ReadFile(tc.tileDirectory, tc.options.Icon)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(iconPNG), nil
}

func (tc *templateContext) version() (string, error) {
	if tc.options.VersionValue != "" {
		return strings.TrimSpace(tc.options.VersionValue), nil
	}
	fileContent, err := fs.ReadFile(tc.tileDirectory, tc.options.VersionFile)
	if err != nil {
		return "", fmt.Errorf("failed to read version: %w", err)
	}
	return strings.TrimSpace(string(fileContent)), nil
}

func (tc *templateContext) stemcellFromKilnfileLock() (string, error) {
	return encodeJSONString(struct {
		OS      string `json:"os"`
		Version string `json:"version"`
	}{
		OS:      tc.kilnfileLock.Stemcell.OS,
		Version: tc.kilnfileLock.Stemcell.Version,
	}, nil)
}

func createMetadataFunc(m namedParts, tc *templateContext) func(string) (string, error) {
	return func(name string) (string, error) {
		return nodeToJSONString(getMetadataFileFromMap(name, m, tc))
	}
}

func (tc *templateContext) releaseFromKilnfileLock(name string) (string, error) {
	lock, err := tc.kilnfileLock.FindReleaseWithName(name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve release %s: %w", name, err)
	}
	localFilePath := path.Base(lock.RemotePath)
	if lock.RemoteSource == cargo.ReleaseSourceTypeBOSHIO {
		localFilePath = fmt.Sprintf("%s-%v.tgz", lock.Name, lock.Version)
	}
	return encodeJSONString(struct {
		// these fields must be ordered alphabetically
		File    string `json:"file"`
		Name    string `json:"name"`
		SHA1    string `json:"sha1"`
		Version string `json:"version"`
	}{
		Name:    lock.Name,
		Version: lock.Version,
		SHA1:    lock.SHA1,
		File:    localFilePath,
	}, err)
}

func nodeToJSONString(node *yaml.Node, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return convertResultToString(wrapErrorOnError(yamlconv.ToJSON(nil, node))(func(error) error {
		return fmt.Errorf("failed to encode as JSON: %w", err)
	}))
}

func (tc *templateContext) variable(name string) (string, error) {
	return encodeJSONString(getValueFromMap(tc.options.Variables, name))
}

func getMetadataFileFromMap(name string, namedParts map[string]part, tc *templateContext) (*yaml.Node, error) {
	p, found := namedParts[name]
	if !found || (p.node != nil && p.node.IsZero()) {
		return nil, fmt.Errorf("no metadata matches %q", name)
	}
	interpolated, err := interpolate(tc, p)
	if err != nil {
		return nil, err
	}
	return interpolated, nil
}

func parseAndPreprocessMetadataPart(dir fs.FS, tileName, root string, result map[string]part) error {
	_, err := fs.Stat(dir, root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fs.WalkDir(dir, root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch path.Ext(p) {
		default:
			return nil
		case ".yml", ".yaml":
		}
		buf, err := fs.ReadFile(dir, p)
		if err != nil {
			return fmt.Errorf("failed to read instance group file ")
		}
		buf, err = preProcess(tileName, path.Base(p), buf)
		if err != nil {
			return err
		}
		err = decodeNamedMetadata(bytes.NewReader(buf), p, result)
		if err != nil {
			return fmt.Errorf("failed to decode %s: %w", p, err)
		}
		return nil
	})
}

func encodeJSONString[T any](data T, err error) (string, error) {
	if err != nil {
		return "", err
	}
	return convertResultToString(wrapErrorOnError(json.Marshal(data))(func(error) error {
		return fmt.Errorf("failed to encode as JSON: %w", err)
	}))
}

func wrapErrorOnError[T any](result T, err error) func(fn func(err error) error) (T, error) {
	return func(fn func(err error) error) (T, error) {
		var zero T
		if err != nil {
			return zero, fn(err)
		}
		return result, err
	}
}

func convertResultToString[T ~string | []byte](res T, err error) (string, error) {
	return string(res), err
}

func getValueFromMap[K comparable, V any](m map[K]V, key K) (V, error) {
	var zero V
	k, found := m[key]
	if !found {
		return zero, fmt.Errorf("not found")
	}
	return k, nil
}

func decodeNamedMetadata(r io.Reader, fileName string, result namedParts) error {
	dec := yaml.NewDecoder(r)
	for documentIndex := 0; ; documentIndex++ {
		var n yaml.Node
		err := dec.Decode(&n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if documentIndex > 0 {
				return fmt.Errorf("failed to decode document %d: %w", documentIndex, err)
			}
			return err
		}
		if n.IsZero() || len(n.Content) == 0 {
			continue
		}
		err = result.add(fileName, &n)
		if err != nil {
			return err
		}
	}
}

func crossProduct[T string | constraints.Float | constraints.Integer | constraints.Complex](a, b []T) []T {
	res := make([]T, 0, len(a)*len(b))
	for _, av := range a {
		for _, bv := range b {
			res = append(res, av+bv)
		}
	}
	return res
}

const (
	repositoryGitSHAVariableName = "metadata-git-sha"
)

func SetGitMetaDataSHA(o *Options, repositoryDirectory string) error {
	if o.Variables == nil {
		o.Variables = make(Variables)
	}
	if _, found := o.Variables[repositoryGitSHAVariableName]; found {
		return nil
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	gitStatus := exec.Command("git", "status", "--porcelain")
	gitStatus.Dir = repositoryDirectory
	err := gitStatus.Run()
	if err != nil {
		if gitStatus.ProcessState.ExitCode() == 1 {
			o.Variables[repositoryGitSHAVariableName] = fmt.Sprintf("DEVELOPMENT-%d", time.Now().Unix())
			return nil
		}
		return fmt.Errorf("failed to run git status")
	}
	var out bytes.Buffer
	gitRevParseHead := exec.Command("git", "rev-parse", "HEAD")
	gitRevParseHead.Dir = repositoryDirectory
	gitRevParseHead.Stdout = &out
	err = gitRevParseHead.Run()
	if err != nil {
		return fmt.Errorf("failed to get HEAD revision hash: %w", err)
	}
	o.Variables[repositoryGitSHAVariableName] = strings.TrimSpace(out.String())
	return nil
}

func regexReplaceAll(expression, inputString, replaceString string) (string, error) {
	re, err := regexp.Compile(expression)
	if err != nil {
		return "", err
	}
	return re.ReplaceAllString(inputString, replaceString), nil
}

func sortKeysAlphabetically(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode:
		sortKeysAlphabetically(n.Content[0])
	case yaml.MappingNode:
		sort.Sort((*sortKeysAlphabeticallyImpl)(n))
		for i := 1; i < len(n.Content); i += 2 {
			sortKeysAlphabetically(n.Content[i])
		}
	case yaml.SequenceNode:
		for i := 0; i < len(n.Content); i++ {
			sortKeysAlphabetically(n.Content[i])
		}
	}
}

type sortKeysAlphabeticallyImpl yaml.Node

func (node sortKeysAlphabeticallyImpl) Len() int {
	return len(node.Content) / 2
}

func (node sortKeysAlphabeticallyImpl) Less(i, j int) bool {
	return strings.Compare(node.Content[i*2].Value, node.Content[j*2].Value) < 1
}

func (node sortKeysAlphabeticallyImpl) Swap(i, j int) {
	i *= 2
	j *= 2
	node.Content[i], node.Content[j] = node.Content[j], node.Content[i]
	node.Content[i+1], node.Content[j+1] = node.Content[j+1], node.Content[i+1]
}

func removeUnderscoreFromIntegers(node *yaml.Node) {
	_ = yamlnode.Walk(node, func(node *yaml.Node) error {
		if node.Tag == "!!int" {
			node.Value = strings.ReplaceAll(node.Value, "_", "")
		}
		return nil
	}, yaml.ScalarNode)
}

package carvel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pivotal-cf/kiln/internal/carvel/models"
	"github.com/pivotal-cf/kiln/pkg/cargo"

	"github.com/hashicorp/go-version"
	"gopkg.in/yaml.v3"
)

// Baker transforms an imgpkg bundle and tile metadata into a BOSH release
// and kiln-compatible tile structure that can be baked into a .pivotal file.
type Baker interface {
	Bake(source string) error
	BakeFromLockfile(source string, releaseLock cargo.BOSHReleaseTarballLock, localTarball string) error
	KilnBake(destination string) error
	GetName() string
	GetVersion() (string, error)
	GetReleaseTarball() (string, error)
	SetWriter(w io.Writer)
	SetProgressWriter(w io.Writer)
}

// NewBaker creates a new Baker for transforming imgpkg bundles into BOSH releases.
func NewBaker() Baker {
	return &baker{
		writer:         io.Discard,
		progressWriter: io.Discard,
	}
}

type baker struct {
	metadata            models.Metadata
	source, destination string
	writer              io.Writer
	progressWriter      io.Writer
}

func (b *baker) KilnBake(destination string) error {
	b.progress("Assembling final .pivotal file...")
	cmd := exec.Command("kiln",
		"bake",
		"--skip-fetch",
		"--output-file", destination,
	)
	cmd.Dir = b.destination
	out, err := cmd.CombinedOutput()
	b.log(string(out))
	if err != nil {
		b.log("failed to invoke kiln: " + string(out))
		return err
	}

	return nil
}

func (b *baker) Bake(source string) error {
	b.source = source
	b.destination = path.Join(source, ".carvel-tile")

	b.progress("Reading tile metadata from " + path.Join(source, "base.yml"))
	yamlPath := path.Join(source, "base.yml")
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlData, &b.metadata)
	if err != nil {
		return err
	}

	ver, err := b.GetVersion()
	if err != nil {
		return err
	}
	b.progress(fmt.Sprintf("Tile: %s version %s (metadata_version %s)", b.metadata.Name, ver, b.metadata.MetadataVersion))

	metadataVersion, err := version.NewVersion(b.metadata.MetadataVersion)
	if err != nil {
		return err
	}
	minVersion, _ := version.NewVersion("3.2.0")
	if metadataVersion.LessThan(minVersion) {
		return errors.New("tile metadata_version too old for kubernetes support (must be >=3.2.0)")
	}

	b.progress("Generating BOSH release structure...")
	err = b.generateBoshReleaseDir()
	if err != nil {
		b.log(err.Error())
		return err
	}

	b.progress("Generating tile layout in " + b.destination)
	err = b.generateOutputTile()
	if err != nil {
		b.log(err.Error())
		return err
	}

	return nil
}

func (b *baker) BakeFromLockfile(source string, releaseLock cargo.BOSHReleaseTarballLock, localTarball string) error {
	b.source = source
	b.destination = path.Join(source, ".carvel-tile")

	b.progress("Reading tile metadata from " + path.Join(source, "base.yml"))
	yamlPath := path.Join(source, "base.yml")
	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlData, &b.metadata)
	if err != nil {
		return err
	}

	ver, err := b.GetVersion()
	if err != nil {
		return err
	}
	b.progress(fmt.Sprintf("Tile: %s version %s (metadata_version %s)", b.metadata.Name, ver, b.metadata.MetadataVersion))

	if releaseLock.Name != b.metadata.Name {
		return fmt.Errorf("lockfile release name %q does not match tile name %q", releaseLock.Name, b.metadata.Name)
	}

	err = os.RemoveAll(b.destination)
	if err != nil {
		return err
	}
	err = os.MkdirAll(b.destination, 0755)
	if err != nil {
		return err
	}

	b.progress("Generating tile layout in " + b.destination)
	err = b.generateBaseYaml()
	if err != nil {
		return err
	}
	err = b.copyFiles()
	if err != nil {
		return err
	}
	err = b.generateJobFiles()
	if err != nil {
		return err
	}
	err = b.generateInstanceGroupFiles()
	if err != nil {
		return err
	}
	err = b.generateRuntimeConfigs()
	if err != nil {
		return err
	}

	releasesDir := path.Join(b.destination, "releases")
	err = os.MkdirAll(releasesDir, 0755)
	if err != nil {
		return err
	}

	destTarball := path.Join(releasesDir, b.metadata.Name+"-"+ver+".tgz")

	b.progress("Copying cached BOSH release from " + localTarball)
	b.log("copying cached BOSH release from " + localTarball)
	err = copyFileContents(localTarball, destTarball)
	if err != nil {
		return fmt.Errorf("failed to copy cached release tarball: %w", err)
	}

	return nil
}

func (b *baker) GetReleaseTarball() (string, error) {
	ver, err := b.GetVersion()
	if err != nil {
		return "", err
	}
	tarball := path.Join(b.destination, "releases", b.metadata.Name+"-"+ver+".tgz")
	if _, err := os.Stat(tarball); err != nil {
		return "", fmt.Errorf("release tarball not found at %s: %w", tarball, err)
	}
	return tarball, nil
}

func (b *baker) GetName() string {
	return b.metadata.Name
}

func (b *baker) GetVersion() (string, error) {
	re := regexp.MustCompile(`\s+`)

	// Replace all occurrences of whitespace with an empty string
	versionNoSpace := re.ReplaceAllString(b.metadata.ProductVersion, "")
	if versionNoSpace != `$(version)` {
		return versionNoSpace, nil
	} else {
		// find the version from a "version" file
		version, err := os.ReadFile(path.Join(b.source, "version"))
		return strings.Trim(string(version), " \t\n\r"), err
	}
}

func (b *baker) SetWriter(w io.Writer) {
	b.writer = w
}

func (b *baker) SetProgressWriter(w io.Writer) {
	b.progressWriter = w
}

func (b *baker) log(message string) {
	_, _ = fmt.Fprintln(b.writer, message)
}

func (b *baker) progress(message string) {
	_, _ = fmt.Fprintln(b.progressWriter, message)
}

func (b *baker) generateBoshReleaseDir() error {
	dirName := path.Join(b.source, ".boshrelease")
	err := os.RemoveAll(dirName)
	if err != nil {
		return err
	}

	b.progress("  Initializing BOSH release")
	commands := []*exec.Cmd{
		exec.Command("bosh", "init-release", "--dir="+dirName),
		exec.Command("bosh", "add-blob", "--dir="+dirName, path.Join(b.source, "bundle.tar"), "imgpkg/bundle.tar"),
		exec.Command("bosh", "generate-package", "--dir="+dirName, "registry-data"),
		exec.Command("bosh", "generate-job", "--dir="+dirName, "registry-data"),
	}
	for _, cmd := range commands {
		b.log("executing " + cmd.String())
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}

		b.log("output: " + string(out))
	}

	// Now populate the specs for packages and jobs
	fileContents := map[string]string{
		"packages/registry-data/packaging": `set -eu
mkdir -p ${BOSH_INSTALL_TARGET}/imgpkg
cp imgpkg/*.tar ${BOSH_INSTALL_TARGET}/imgpkg
`,
		"packages/registry-data/spec": `---
name: registry-data
dependencies: []
files:
- imgpkg/bundle.tar
`,
	}
	for outpath, contents := range fileContents {
		err = os.WriteFile(path.Join(dirName, outpath), []byte(contents), 0644)
		if err != nil {
			return err
		}
	}

	registryDataTemplates := ""
	registryDataProperties := ""

	b.progress("  Configuring package installs")
	for _, entry := range b.metadata.PackageInstalls {
		entry = strings.Trim(entry, "$() ")
		entry = strings.TrimPrefix(entry, "package")
		entry = strings.Trim(entry, `"' `)

		b.progress("    - " + entry)
		b.log("looking for package install: " + entry)

		// find this entry in the packageinstalls directory
		matches, err := filepath.Glob(path.Join(b.source, "packageinstalls/*.yml"))
		if err != nil {
			return err
		}

		for _, match := range matches {
			yamlData, err := os.ReadFile(match)
			if err != nil {
				return err
			}

			var pi models.PackageInstall
			err = yaml.Unmarshal(yamlData, &pi)
			if err != nil {
				return err
			}
			if pi.Name != entry {
				continue
			}

			b.log("found " + pi.Name + " at " + match)
		}

		registryDataTemplates += fmt.Sprintf("  packageinstalls/%s.yml.erb: packageinstalls/%s.yml\n", entry, entry)

		registryDataProperties += "  " + entry + ":\n"
		registryDataProperties += `    name:
      description: "package name"
    version:
      description: "package version"
    values:
      description: "values.yml contents"
`

		if err = os.MkdirAll(path.Join(dirName, "jobs", "registry-data", "templates", "packageinstalls"), 0755); err != nil {
			return err
		}

		manifestTemplate := generateManifestTemplate(entry)

		err = os.WriteFile(
			path.Join(dirName, "jobs", "registry-data", "templates", "packageinstalls", entry+".yml.erb"),
			[]byte(manifestTemplate),
			0644,
		)
		if err != nil {
			return err
		}
	}

	registryDataSpec := `---
name: registry-data
templates:
` + registryDataTemplates +
		`packages:
- registry-data
consumes:
- name: cluster
  type: cluster-info
  optional: true
properties:
` + registryDataProperties

	err = os.WriteFile(path.Join(dirName, "jobs", "registry-data", "spec"), []byte(registryDataSpec), 0644)
	if err != nil {
		return err
	}

	return nil
}

func generateManifestTemplate(entry string) string {
	return `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: <%= p("` + entry + `.name") %>-sa
  namespace: <%= link("cluster").p("content-namespace") rescue "default" %>
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: <%= p("` + entry + `.name") %>-sa-cluster-role
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: <%= p("` + entry + `.name") %>-sa-cluster-role-binding
subjects:
- kind: ServiceAccount
  name: <%= p("` + entry + `.name") %>-sa
  namespace: <%= link("cluster").p("content-namespace") rescue "default" %>
roleRef:
  kind: ClusterRole
  name: <%= p("` + entry + `.name") %>-sa-cluster-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: Secret
metadata:
  name: <%= p("` + entry + `.name") %>-values
  namespace: <%= link("cluster").p("content-namespace") rescue "default" %>
type: Opaque
stringData:
  values.yaml: |
<% require 'yaml' %>
<%
  values = p("` + entry + `.values")
  values = YAML.load(values) if values.is_a?(String)
  # Inject namespace from BOSH link into context
  if values.is_a?(Hash) && values["context"].is_a?(Hash)
    values["context"]["namespace"] = link("cluster").p("content-namespace") rescue "default"
  end
%>
<%= YAML.dump(values).split("\n").map { |line| "    " + line }.join("\n") %>
---
apiVersion: packaging.carvel.dev/v1alpha1
kind: PackageInstall
metadata:
  name: <%= p("` + entry + `.name") %>
  namespace: <%= link("cluster").p("content-namespace") rescue "default" %>
spec:
  serviceAccountName: <%= p("` + entry + `.name") %>-sa
  packageRef:
    refName: <%= p("` + entry + `.name") %>
    versionSelection:
      constraints: <%= p("` + entry + `.version") %>
  values:
  - secretRef:
      name: <%= p("` + entry + `.name") %>-values
`
}

func (b *baker) generateOutputTile() error {
	err := os.RemoveAll(b.destination)
	if err != nil {
		return err
	}

	err = os.MkdirAll(b.destination, 0755)
	if err != nil {
		return err
	}

	b.progress("  Generating base.yml")
	err = b.generateBaseYaml()
	if err != nil {
		return err
	}

	b.progress("  Copying forms, properties, and static assets")
	err = b.copyFiles()
	if err != nil {
		return err
	}

	err = b.generateJobFiles()
	if err != nil {
		return err
	}

	err = b.generateInstanceGroupFiles()
	if err != nil {
		return err
	}

	b.progress("  Generating runtime configs")
	err = b.generateRuntimeConfigs()
	if err != nil {
		return err
	}

	b.progress("  Creating BOSH release tarball (this may take a while)...")
	err = b.createBoshRelease()
	if err != nil {
		return err
	}

	return nil
}

func (b *baker) generateBaseYaml() error {
	meta := models.MetadataOut{}
	meta.Name = b.metadata.Name
	meta.Label = b.metadata.Label
	meta.IconImage = b.metadata.IconImage
	meta.ProductVersion = b.metadata.ProductVersion
	meta.MetadataVersion = b.metadata.MetadataVersion
	meta.Rank = b.metadata.Rank
	meta.Serial = b.metadata.Serial
	meta.CompatibleKubernetesDistributions = b.metadata.CompatibleKubernetesDistributions
	meta.FormTypes = b.metadata.FormTypes
	meta.PropertyBlueprints = b.metadata.PropertyBlueprints
	meta.Variables = b.metadata.Variables
	meta.MinimumVersionForUpgrade = b.metadata.MinimumVersionForUpgrade
	meta.RequiresKubernetes = true
	// stemcell criteria are dummy data that OM will ignore when the tile is folded into
	// TKR, but we need them as kiln inputs.
	meta.StemcellCriteria.Os = "ubuntu-jammy"
	meta.StemcellCriteria.Version = "1.446"
	meta.InstanceGroups = []string{}
	meta.RuntimeConfigs = []string{
		`$( runtime_config "` + b.metadata.Name + `-pkgr" )`,
	}

	// we will use the tile name and version as the bosh release name and version.
	meta.Releases = []string{
		`$( release "` + b.metadata.Name + `" )`,
	}

	yamlData, err := yaml.Marshal(&meta)
	if err != nil {
		return err
	}
	err = os.WriteFile(path.Join(b.destination, "base.yml"), yamlData, 0644) // 0644 sets file permissions
	if err != nil {
		return err
	}
	return nil
}

// copyfiles schleps all the files that we can collect from the source directory without
// modification:
// - variables, properties and forms
// - the icon file
// - the version file
func (b *baker) copyFiles() error {
	for _, subdir := range []string{"bosh_variables", "forms", "properties"} {
		info, err := os.Stat(path.Join(b.source, subdir))
		if err == nil && info.IsDir() {
			err = os.CopyFS(path.Join(b.destination, subdir), os.DirFS(path.Join(b.source, subdir)))
			if err != nil {
				return err
			}
		}
	}

	for _, fn := range []string{"icon.png", "version"} {
		info, statErr := os.Stat(path.Join(b.source, fn))
		if statErr == nil && !info.IsDir() {
			if err := copyFileContents(path.Join(b.source, fn), path.Join(b.destination, fn)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *baker) generateRuntimeConfigs() error {
	err := os.MkdirAll(path.Join(b.destination, "runtime_configs"), 0755)
	if err != nil {
		return err
	}

	registryDataProps := map[string]models.PackageInstallProps{}

	// we need one PackageInstall for each entry in the metadata.
	for _, entry := range b.metadata.PackageInstalls {
		entry = strings.Trim(entry, "$() ")
		entry = strings.TrimPrefix(entry, "package")
		entry = strings.Trim(entry, `"' `)

		// find this entry in the packageinstalls directory
		matches, err := filepath.Glob(path.Join(b.source, "packageinstalls/*.yml"))
		if err != nil {
			return err
		}

		found := false
		for _, match := range matches {
			yamlData, err := os.ReadFile(match)
			if err != nil {
				return err
			}

			var pi models.PackageInstall
			err = yaml.Unmarshal(yamlData, &pi)
			if err != nil {
				return err
			}
			if pi.Name != entry {
				continue
			}

			found = true
			registryDataProps[entry] = models.PackageInstallProps{
				Name:    pi.PackageName,
				Version: pi.PackageVersion,
				Values:  pi.Values,
			}
		}
		if !found {
			return errors.New("package install not found: " + entry)
		}
	}

	registryDataJob := models.Job{
		Name:       "registry-data",
		Release:    b.metadata.Name,
		Properties: registryDataProps,
	}

	inner := models.RuntimeConfigInner{
		Releases: []string{
			`$( release "` + b.metadata.Name + `" )`,
		},
		Addons: []models.Addon{
			{
				Name: b.metadata.Name + "-pkgr",
				Include: models.Inclusion{
					Deployments: []string{
						`(( ..` + b.metadata.Name + `.deployment_name ))`,
					},
					Jobs: []models.Job{
						{Name: "install-package-repository", Release: "tanzu-content"},
						{Name: "install-packages", Release: "tanzu-content"},
					},
				},
				Jobs: []models.Job{
					registryDataJob,
				},
			},
		},
	}
	yamlData, err := yaml.Marshal(&inner)
	if err != nil {
		return err
	}
	rc := models.RuntimeConfigOuter{
		Name:          b.metadata.Name + "-pkgr",
		RuntimeConfig: string(yamlData),
	}

	yamlData, err = yaml.Marshal(&rc)
	if err != nil {
		return err
	}
	err = os.WriteFile(path.Join(b.destination, "runtime_configs", b.metadata.Name+"-pkgr.yml"), yamlData, 0644)
	if err != nil {
		return err
	}

	return nil
}

// generateInstanceGroups creates an empty instance group folder
func (b *baker) generateInstanceGroupFiles() error {
	err := os.MkdirAll(path.Join(b.destination, "instance_groups"), 0755)
	if err != nil {
		return err
	}

	return nil
}

// generateJobFiles creates an empty jobs folder
func (b *baker) generateJobFiles() error {
	err := os.MkdirAll(path.Join(b.destination, "jobs"), 0755)
	if err != nil {
		return err
	}

	return nil
}

func (b *baker) createBoshRelease() error {
	err := os.MkdirAll(path.Join(b.destination, "releases"), 0755)
	if err != nil {
		return err
	}

	version, err := b.GetVersion()
	if err != nil {
		return err
	}

	dirName := path.Join(b.source, ".boshrelease")
	cmd := exec.Command("bosh",
		"create-release",
		"--dir="+dirName,
		"--force",
		"--name", b.metadata.Name,
		"--version", version,
		"--tarball", path.Join(b.destination, "releases", b.metadata.Name+"-"+version+".tgz"))
	b.log("executing " + cmd.String())
	out, err := cmd.CombinedOutput()
	b.log("output: " + string(out))
	if err != nil {
		return err
	}

	return nil
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

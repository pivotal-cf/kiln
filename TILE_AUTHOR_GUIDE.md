# Tile Author Guide

> Kiln provides utilities and conventions for maintaining the source to build and release ".pivotal" files.

_The shell script examples use [kiln 0.85.0](https://github.com/pivotal-cf/kiln/releases/tag/v0.85.0), [yq 4.34.1](https://github.com/mikefarah/yq), and **curl 7.88.1**._

### Table of Contents
- [Authoring Product Template Parts](#authoring-product-template-parts)
- [Managing BOSH Release Tarballs](#bosh-release-tarballs)
  - [Kilnfile BOSH Release Specification and Locking](#bosh-release-sources)
    - Sources
      - [BOSH.io](#release-source-boshio)
      - [GitHub releases](#release-source-github)
      - [Build Artifactory](#release-source-artifactory)
      - [AWS S3](#release-source-s3)
      - [Local Files](#release-source-directory)
  - BOSH release compilation
- Stemcell Version Management
- Testing
- Tile release note Generation
- PivNet Release Publication
- Provides importable utilities for Tile Authors

Again, see [hello-tile](https://github.com/crhntr/hello-tile) (non-VMware employees) or the TAS repo (VMware employees) for tile source code that kiln "just works" with.

## Bootstrapping a tile repository

The following code bootstraps Product template parts from an empty directory working tile; it creates conventional defaults to start/standardize your tile authorship journey with Kiln.
It does not generate source for a working tile.
```shell
# Make directories and keep them around
mkdir -p bosh_variables forms instance_groups jobs migrations properties releases runtime_configs
touch {bosh_variables,forms,instance_groups,jobs,migrations,properties,releases,runtime_configs}/.gitkeep

# Add a tile image
curl -L -o icon.png "https://github.com/crhntr/hello-tile/blob/main/icon.png?raw=true"

# Add a version file (this is the default location Kiln reads from)
echo '0.1.0' > 'version'

# Create a "base.yml" with some minimal fields
# See documentation here: `https://docs.pivotal.io/tiledev/2-9/property-template-references.html`
cat << EOF > base.yml
name: my-tile-name
label: ""
description: ""
icon_image: $( icon )
metadata_version: "3.0.0"
minimum_version_for_upgrade: 0.0.0
product_version: $( version )
provides_product_versions:
  - name: hello
    version: $( version )
rank: 1
serial: false
releases: []
stemcell_criteria: []
job_types: []
runtime_configs: []
property_blueprints: []
form_types: []
EOF
```

<details>
<summary>PPE TODO</summary>

Create a `kiln init` command that actually makes a working tile.

</details>

## <a id="authoring-product-template-parts"></a> Authoring Product Template Parts

A tile is a zip file with a ".pivotal" suffix.
The zip file must have a YAML file in the `metadata` subdirectory of the tile.
The file specification is documented in the [**Property and Template References** page in the Ops Manager Tile Developer Guide](https://docs.pivotal.io/tiledev/2-9/property-template-references.html).

While you can write your entire Product Template in a single file, breaking up metadata parts into different yaml files and directories makes it easier to find configuration.
Kiln expects certain directories to contain Product Template (metadata) parts.

- `./forms` should contain YAML files that have [form_types](https://docs.pivotal.io/tiledev/2-9/property-template-references.html#form-properties)
- `./instance_groups` should contain YAML files that have [job_types](https://docs.pivotal.io/tiledev/2-9/property-template-references.html#job-types)
- `./jobs` should contain YAML files that have  [job templates](https://docs.pivotal.io/tiledev/2-9/property-template-references.html#job-template) and may contain a [job mainfest](https://docs.pivotal.io/tiledev/2-9/property-template-references.html#job-manifest)
- `./migrations` should contain Javascript files that are migrations (TODO add link to specification)
- `./properties` should contain YAML files that have [property-blueprints](https://docs.pivotal.io/tiledev/2-9/property-template-references.html#property-blueprints)
- `./releases` should be an empty directory (maybe containing .gitkeep) and will be where Kiln writes BOSH Release Tarballs
- `./runtime_configs` should contain YAML files that have [runtime configs](https://docs.pivotal.io/tiledev/2-9/runtime-config.html)

Also add the following files
- `base.yml` should be the entrypoint for the manifest template.
- `version` should contain the tile version
- `icon.png` should be the tile image

While you can use other directory and filenames and pas those to Kiln,
you would have a better experience if you follow the above naming conventions.

### Adding product template parts to your Metadata

#### Product Template baking functions

Each product template part has a "name" field.
When you `kiln bake`, the contents of `base.yml` go through a few templating steps.
One of those steps exposes the following functions.
These functions will read, interpolate, and format metadata parts for use in your tile's metadata.yml.

You can reference the part in base.yml by using the follwing template functions:
- `bosh_variable` reads, interpolates, and formats a product template part from `./bosh_variables`
- `form` reads, interpolates, and formats a product template part from `./forms`
- `property` reads, interpolates, and formats a product template part from `./properties`
- `release` reads, interpolates, and formats a BOSH release data from either the `./releases` directory or the Kilnfile.lock
- `stemcell` reads, interpolates, and formats a Stemcell Criteria data from the Kilnfile.lock
- `version` returns the contents of the `./version` file
- `variable` finds the named variables from either a `--variables-file` or `--variable`
- `icon` returns the base64 encoded content of `icon.png`
- `instance_group` reads, interpolates, and formats a product template part from `./instance_groups`
- `job` reads, interpolates, and formats a product template part from `./jobs`
- `runtime_config` reads, interpolates, and formats a product template part from `./runtime_configs`

Other functions:
- `tile` _TODO function documentation_
- `select`  _TODO function documentation_
- `regexReplaceAll`  _TODO function documentation_

See the [crhntr/hello-tile/base.yml](https://github.com/pivotal/hello-tile/blob/c6b59dcb1118c9b2f5d4fbf836842ce4033baa80/base.yml#L29C1-L30C1) for some use of the above functions.
The property definition in [crhntr/hello-tile/properties/hello.yml](https://github.com/pivotal/hello-tile/blob/c6b59dcb1118c9b2f5d4fbf836842ce4033baa80/properties/hello.yml#L2-L5)
in referenced in `base.yml` using `$( property "port" )`.
Most other product template part functions behave similarly.

## <a id="bosh-release-tarballs"></a> Managing BOSH Release Tarballs

`kiln fetch` downloads BOSH Release Tarballs from any of the following "sources"
and puts them in a "./releases" directory.

Before Kiln can help you manage the BOSH Releases you put in the tile, you need to upload BOSH Release tarballs to a place accessable to Kiln.

See the sources below to decide which is right for your release.

Unlike Tile Generator.
Kiln does not create releases.
The Kiln way is to 
- have BOSH releases each have their own source control
- have CI build and upload (finalized) BOSH release tarballs somewhere
- have a tile source repository
  - in the tile source repository specify how Kiln can get the BOSH release tarballs

While the following examples start from empty directories and mostly use S3 and BOSH.io.
There are similar simple scripts for a small test tile illustrating similar usage patterns to the follwoing example.
See [hello-tile](https://github.com/crhntr/hello-tile).
Hello Tile consumes a single custom BOSH Release, [hello-release](https://github.com/crhntr/hello-release), from a GitHub release.
It does not upload the release to PivNet but adds the built tile to a GitHub Release.

#### <a id="kiln-fetch-example"></a> **EXAMPLE** Using Kiln to Download BOSH Release Tarballs
This starts from an empty directory and downloads the latest BPM release from bosh.io.
Note, the Kilnfile and Kilnfile.lock (unfortunately/frustratingly) must be created manually.

```sh
# Create and go into an empty directory
mkdir -p /tmp/try-kiln-fetch
cd !$
mkdir -p releases
touch release/.gitkeep # not required but a good idea

# Hack a Kilnfile and Kilnfile lock
echo '{"release_sources": [{type: bosh.io}], "releases": [{"name": "bpm"}]}' > Kilnfile
yq '{"releases": [. | {"name": "bpm", "version": .version, "sha1": .sha, "remote_source": "bosh.io", "remote_path": .remote_path}]}' <(kiln find-release-version --release=bpm) > Kilnfile.lock

# Call Kiln fetch
kiln fetch

# See the fetched release
stat releases/bpm*.tgz
```

The files should look something like these
```yaml
# Expected Kilnfile
release_sources:
  - type: "bosh.io"
releases:
  - name: bpm
```

```yaml
# Expected Kilnfile.lock
releases:
  - name: bpm
    version: "1.2.3"
    sha1: "ad12bb4e1d2c0b94ef679670a99adaff920850d0"
    remote_source: bosh.io
    remote_path: "https://bosh.io/d/github.com/cloudfoundry/bpm-release?v=1.2.3"
```

<details>
<summary>PPE TODO</summary>

The YQ expressions are a hack to get this to work from an empty directory.
We need to improve this process.
Kiln fetch was built around an existing "assets.lock";
the developer experience for starting from an empty directory is not polished.
</details>

#### **EXAMPLE** Using Kiln to update BOSH Release Tarballs

Updating a release in a lock file requires two kiln commands.

Please follow the ["Kiln Fetch Example"](#kiln-fetch-example) before following this one.

```sh
# (optional) Add a version constraint to the Kilnfile.
# This shows how Kiln will select a version that matches a constrint.
yq '(.releases[] | select(.name == "bpm")) |= .version = "~1.1"' Kilnfile

# Find a new BOSH Release Tarball version (on bosh.io)
export NEW_RELEASE_VERSION="$(kiln find-release-version --release=bpm | yq '.version')"
echo "Kiln found: bpm/${NEW_RELEASE_VERSION}"

# Update the Kilnfile.lock with the new version
kiln update-release --version="${NEW_RELEASE_VERSION}" --name="bpm"
```

The syntax for BOSH Release Tarball version constraints is [Masterminds/semver](https://github.com/Masterminds/semver#checking-version-constraints).
Other parts of the Cloud Foundry ecosystem use [blang/semver](https://github.com/blang/semver#ranges).
If you get unexpected results, this difference may be the cause.
For simple version constraints they are similar enough.

`kiln update-release` ignores the content of Kilnfile.
This can cause `kiln validate` to fail when a version passed to `kiln update-release` does not match the constraint in the Kilnfile. _This behavior may/should change._

<details>
<summary>PPE TODO</summary>

This developer experience needs work IMO.

The release name flag difference is awkward.
In `find-release-version`, the flag is `--release`;
In `kiln update-release`, the flag is `--name`.

Maybe this should be one command and an optional flag
- `kiln update-release`       →`kiln update-bosh-release bpm`
- `kiln find-release-version` → `kiln update-bosh-release --dry-run bpm`

</details>

### Release Sources

While different credentials per release source element are currently supported.
I would recommend one set of credentials per release source type.

In the Kilnfile.lock, BOSH release tarballs lock elements have a few fields.
- `name`: The BOSH release name
- `version`: The BOSH release version
- `sha1`: The sha1 sum of the BOSH release tarball
- `remote_source`: The identifier of the BOSH Release tarball source specified in the Kilnfile where the tarball is stored
- `remote_path`: A source specific string to identify where the tarball is. This _may_ be a URL.

##### <a id='kilnfile-secrets'></a> Kilnfile Secrets

A Kilnfile (currently) specifies all the strings required to configure a BOSH Release tarball source.
This includes secrets.
While you _can_ just add the secrets to the Kilnfile, don't.
The Kilnfiles go through an initial templating step before being parsed.
Please don't use this for anything but secret injection.
Most kiln commands recieve a `--variable` or a `--variables-file` flag.

To use the `--variable` flag run something like this:

`kiln fetch --variable=fruit=banana`

In your Kilnfile use the fruit variable like this.

```
release_source:
  - some_field: $(variable "banana")
  - some_field: "$(variable "banana")"
  - some_field: '$(variable "banana")'
```

##### <a id='release-source-id'></a> Explicit Release Source ID

Please set an explicit ID for each release source.
Kiln has fall-back behavior to use other fields to identify a release source (like bucket for S3 or owner for GitHub...)
but this fall-back behavior can be hard to follow.
Just set `id` on all of your release sources and make mapping releases in the Kilnfile.lock to the release source in Kilnfile easier to follow.

##### <a id='path-templates'></a> Path Templates

While some BOSH release tarball sources use URLs as the `remote_path` in their release locks,
others (S3 and Artifactory) rely on a path template in their configuration.

The `path_template` is uses [`text/template`](https://pkg.go.dev/text/template) and is passed a value with the following type.

```go
package cargo

// PathTemplateData is passed to template.Execute along with the parsed Release Source path template.
// this type is not the real one in the source.
type PathTemplateData struct{
    // Name is set to the BOSH Release name
    Name string
    
    // Version is set to the BOSH Release version
    Version string

    // Name is set to the Kilnfile.lock StemcellCriteria OS value
    StemcellOS string

    // Name is set to the Kilnfile.lock StemcellCriteria Version value
    StemcellVersion string
}
```

Here are some example path templates (the rest of the release source config has been omitted).

```yaml
release_sources:
  - path_template: "bla-bla/{{.Name}}/{{.Version}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
  - path_template: "bla-bla/{{.Name}}/{{.Version}}/{{.Name}}-{{.Version}}.tgz"
  - path_template: "bla-bla/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz"
```

Avoid using other Go text/template actions like `{{if pipeline}}` and the like.

#### <a id="release-source-boshio"></a> BOSH.io

Kiln can only download releases from BOSH.io and can not upload BOSH Releases to BOSH.io.

This release source has minimal configuration.
Just add it to your `release_sources` and you can get releases from [BOSH.io](https://bosh.io/releases/).

```yaml
# Expected Kilnfile.lock
release_sources:
  - type: bosh.io
    id: community # (optional) the default ID for this type is the constant string "bosh.io"
```

The value of `remote_path` in the BOSH release tarball lock is a URL.


#### <a id='release-source-github'></a> GitHub Release Artifacts

Kiln can only download releases from GitHub Releases and can not upload BOSH Releases to BOSH.io.

To download BOSH Release Tarballs from GitHub Releases, add the following
```
release_sources:
  - type: "github"
    id: crhntr # (optional) the default ID in this case is the value of org
    org: "crhntr"
    github_token: $(variable "github_token")
```

You will need one entry per organization.
Some examples are: "pivotal", "cloudfoundry", "pivotal-cf", or your personal/company GitHub username.

You will need to add the following flag to most commands:

```
# Optional helper
export GITHUB_TOKEN="$(gh auth status --show-token 2>&1 | grep 'Token:' | awk '{print $NF}')"

# Example Kiln variable flag
kiln fetch --variable="github_token=${GITHUB_TOKEN}"
```

The value of `remote_path` in the BOSH release tarball lock is a URL.

#### <a id='release-source-artifactory'></a> Build Artifactory

Kiln can fetch and upload releases to/from Build Artifactory.

The release source specification should look like this:

```yaml
release_sources:
  - type: "artifactory"
    id: "official-storage"  # (optional) the default ID for this type is the constant string value "artifactory"
    repo: "some-repository"
    username: "some-username"
    password: "some-password"
    path_template: "some-path-template/tarball.tgz"
```

Note `repo` is not a GitHub repository. It is an Artifactory thing. 

Please see [Path Templates](#path-templates). The value of `remote_path` in the BOSH release tarball lock is part of the path needed to construct a URL to download the release.

#### <a id='release-source-s3'></a> AWS S3

Kiln can fetch and upload releases to/from AWS S3.

```yaml
release_sources:
  - type: "artifactory"
    bucket: "some-bucket"
    id: "legacy-storage"  # (optional) the default ID for this type is the value of bucket
    region: "some-region"
    access_key_id: "some-access-key-id"
    secret_access_key: "some-secret-access-key"
    path_template: "some-path-template/tarball.tgz"
```

Please see [Path Templates](#path-templates). The value of `remote_path` in the BOSH release tarball lock is part of the path needed to make the S3 object name.

#### <a id='release-source-directory'></a> Local tarballs

`kiln bake` adds the BOSH release tarballs in the releases directory to the tile reguardless of if they match the Kilnfile.lock.
Building a tile with arbitrary releases in the tarball is not secure; this behavior should only be used for development not for building production tiles.

#### Default credentials file

You can add a default credentials file to `~/.kiln/credentials.yml` so you don't need to pass variables flags everywhere.
Don't do this with production creds but if you have credentials you can safely write to your disk, consider using this functionality.
The file can look like this
```yaml
# GitHub BOSH release tarball release sources credentials
github_token: some-token

# S3 release BOSH release tarball source credentials
aws_secret_access_key: some-key
aws_access_key_id: some-id

# Artifactory BOSH release tarball release source credentials
artifactory_username: some-username
artifactory_password: some-password
```

### Release Compilation

_WORK IN PROGRESS EXAMPLE_

```sh
# create a tile with the releases you want compiled
kiln bake

# Add an S3 (or Artifactory) release source to your Kilnfile
yq -i '.release_sources = [{"type": "s3", "id": "my_compiled_release_bucket", bucket": "some_bucket", "publishable": true, "access_key_id": "some_id", "secret_access_key": "some_id"}] + .release_sources' Kilnfile

# claim a bosh director and configure the environment
smith claim -p us_4_0
eval "$(smith om)"
eval "$(smith bosh)"

# deploy your Product (the commands should look something like this)
om upload-product --product=tile.pivotal
om configure-product --config=simple_config.yml
om apply-changes --product-name=my-tile-name

# Download Compiled BOSH Releases from the BOSH Director and Upload them to the S3 Bucket or Artifactory
kiln cache-compiled-releases --upload-target-id=my_compiled_release_bucket --name=hello

# Commit and push the changes to Kilnfile.lock
git add -p Kilnfile.lock
git commit -m "compile BOSH Releases with $(yq '.stemcell_criteria.os' Kilnfile.lock)/$(yq '.stemcell_criteria.version' Kilnfile.lock)"
git push origin HEAD
```

### Temporary BOSH Release Tarball Locking

This functionality is likely not helpful for tile authors who only package their own BOSH releases or for those who only package a few BOSH releases.

If you need to pause BOSH Release bumps in your Kilnfile.lock,
you can execute `kiln glaze`.
It sets the BOSH release version (constraint) fields to the semver from the Kilnfile.lock.
[This command has a PR to make it more helpful for TAS/IST/TASW. See the PR here](https://github.com/pivotal-cf/kiln/pull/406).
This effectively pins releaess to block Dependabot updates.
PPE uses this command for TAS/IST/TASW prior to new major versions.

## Stemcell Version Management

`kiln find-stemcell-version` and `kiln update-stemcell`

Find the latest stemcell releases on PivNet. They behave similarly to the bosh release commands above.

_If I remember right, the find-stemcell-version command has a bug where the stemcell criteria version in the Kilnfile is not respected and the result of the command is always the latest version._

## Tile Release Note Generation

If you GitHub BOSH Release tarball sources,
you can generate release notes for your tile that contain release notes for each BOSH release.

This feature requires you to have a previous tile release that used a Kilnfile.lock to specify the BOSH releases packaged.
You pass two references
- the Git Reference or SHA of the commit of the source used to generate the previously published tile
- Git Reference or SHA of the commit of the source used to generate the **next** tile

While you can override the template and the regular expression used to make these notes.
They are quite hard to craft.
I recommend you use the defaults.

`kiln release-notes --update-docs=path-to-release-notes-file/notes.md "${PREVIOUS_RELEASE_SHA}" "${NEXT_RELEASE_SHA}"`

If you omit `--update-docs` the notes will be written to standard out.

## PivNet Release Publication

`kiln publish` does not in-fact publish a tile.
It changes some of the configuration on a previously created PivNet release.
While we use it for TAS, it is not ready/intended to be used by other tiles quite yet.

## Importing Go Source Code [![Go Reference](https://pkg.go.dev/badge/github.com/pivotal-cf/kiln.svg)](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg).

**Note the Kiln repository is pre-1.0.0. While we _try_ to maintain backwards compatablility with the commands. The package API is subject to change without notice.**  

See the [Hello Tile manifest test](https://github.com/pivotal/hello-tile/blob/main/test/manifest/manifest_test.go) to see how to use this in tests.
Follow the conventions you see in hello-tile, and you should be able to run `kiln test`.
The github.com/pivotal-cf/kiln/pkg/proofing/upgrade package can help you detect changes that would require foundation provider/operator) intervention.

`go get github.com/pivotal-cf/kiln/pkg/cargo`

```go
package main

import (
	"log"
	"fmt"
	
	"gopkg.in/yaml.v3"

	"github.com/pivotal-cf/kiln/pkg/cargo"
	"github.com/pivotal-cf/kiln/pkg/tile"
	"github.com/pivotal-cf/kiln/pkg/proofing"
)

func main() {
	// log release versions
	lock, err := cargo.ReadKilnfileLock("Kilnfile.lock")
	if err != nil {
		log.Fatal(err)
	}
	for _, releaseLock := range lock.Releases {
		fmt.Println(releaseLock.Name, releaseLock.Version)
	}

	// print name and rank from tile.pivotal
	manifestBytes, err := tile.ReadMetadataFromFile("tile.pivotal")
	if err != nil {
		log.Fatal(err)
	}
	var productTemplate proofing.ProductTemplate
	err = yaml.Unmarshal(manifestBytes, productTemplate)
	if err != nil {
		log.Fatal(err)
    }
	fmt.Println(productTemplate.Name, productTemplate.Rank)
}
```

## Tile Build Packs

There is an internal VMware intiative to build stuff using TAP and buildpacks.
The Kiln Buildpack can take tile source code and create a tile.
For it to work,
you need to have your BOSH Release Tarballs fetch-able by Kiln (and only using GitHub or BOSH.io release sources)
and it is nice if your bake command not require too many flags (see [Tile Source Conventions](#tile-source-conventions)).

The private repository [kiln buildpack](https://github.com/pivotal/kiln-buildpack) has the Pakito buildpack source.
You can run the acceptance tests with a `TILE_DIRECTORY` environment variable set to your tile source to see if your tile will build with the buildpack.

```
mkdir -p /tmp/try-kiln-buildpack
cd !$

set -e

# Clone the Buildpack source
git clone git@github.com:pivotal/kiln-buildpack
cd kiln-buildpack

# Check the path to your tile directory
stat ${HOME}/workspace/my-tile-source/Kilnfile.lock

# WARNING this does a git clean. This is important to simulate building from source.
# If you have releases fetched already, you won't get an acurate test. 
cd stat ${HOME}/workspace/my-tile-source && git clean -ffd && cd -

# Run the acceptance test against your tile source
TILE_DIRECTORY="${HOME}/workspace/my-tile-source" go test -v --tags=acceptance .

stat /tmp/try-kiln-buildpack/kiln-buildpack/tile.pivotal
```

The buildpack is intended to use on TAP. It is still in early development. 


## Other Tips

### .gitignore

The following are some useful .gitignore contents.

```.gitignore
releases/*.tgz
*.pivotal
```
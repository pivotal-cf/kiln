# Kiln [![release](https://github.com/pivotal-cf/kiln/actions/workflows/release.yml/badge.svg)](https://github.com/pivotal-cf/kiln/actions/workflows/release.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/pivotal-cf/kiln.svg)](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg)

_Kiln bakes tiles_

Kiln helps tile developers build products for [VMware Tanzu Operations Manager](https://network.tanzu.vmware.com/products/ops-manager/). It provides
an opinionated folder structure and templating capabilities. It is designed to be used
both in CI environments and in command-line to produce a tile.

More information for those just getting started can be found in the [Ops Manager Tile Developer Guide](https://docs.vmware.com/en/Tile-Developer-Guide/3.0/tile-dev-guide/index.html) .
Looking at an [example kiln tile](https://github.com/crhntr/hello-tile/tree/main) may also be helpful

## Installation

To install the `kiln` CLI 
- install with Homebrew

  ```shell
  brew tap pivotal-cf/kiln https://github.com/pivotal-cf/kiln
  brew install kiln
  ```

- download from the [releases page](https://github.com/pivotal-cf/kiln/releases)

  ```shell
  export KILN_VERSION
  KILN_VERSION="$(curl -H "Accept: application/vnd.github.v3+json" 'https://api.github.com/repos/pivotal-cf/kiln/releases?per_page=1' | jq -r '.[0].name')"
  curl -L -o kiln "https://github.com/pivotal-cf/kiln/releases/download/${KILN_VERSION}/kiln-darwin-${KILN_VERSION}"
  # check the checksum
  cp kiln "$(go env GOPATH)/bin"
  kiln version
  ```

- build from source
  
  ```shell
  git clone git@github.com:pivotal-cf/kiln.git
  cd kiln
  ./scripts/install.sh
  ```

- copy from a Docker image (to another image)

  ```shell
  docker pull pivotalcfreleng/kiln:latest
  ```

   ```Dockerfile
  FROM pivotalcfreleng/kiln:latest as kiln

  FROM ubuntu
  COPY --from=kiln /kiln /usr/bin/kiln
  CMD /usr/bin/bash
   ```
## Kilnfile

Each tile must have a Kilnfile and Kilnfile.lock. Both are YAML files. Kiln won't generate them for you.

The code for parsing the Kilnfile and Kilnfile.lock exists in this package: [`"github.com/pivotal-cf/kiln/pkg/cargo"`](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg/cargo#Kilnfile).
Although the package interface is not yet stable, we have found importing it directly to be useful in CI or one-off scripts.   

### The Specification [(source)](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg/cargo#KilnfileLock)

This file contains a range of configuration in support of kiln commands.

#### "slug"

This field must be a string.

This field should be populated with the TanzuNet product slug where this tile is published.

It is used by kiln publish.

#### "pre_ga_user_groups"

This field must be a list of strings.

This should be populated with the names of TanzuNet User Groups to attach to a pre-release on TanzuNet.

It is used by kiln publish.

#### "release_sources"

This field must be a list of objects with keys from [`ReleaseSourceConfig`](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg/cargo#ReleaseSourceConfig).
All elements must have a `type` field. 

The values for the `type` (string) field are `"bosh.io"`, `"s3"`, `"github"`, or `"artifactory"`

See `fetch` documentation for more details.

#### "stemcell_critera"

#### "releases"

Each release you want to add to your tile must have an element in this array.
The "name" field with the BOSH Release Name as the value must be set here.
The BOSH Release Name must be identical to the name of the release.

You must set a **"name"** field with the BOSH Release Name as the value must be set here.
The BOSH Release Name must be identical to the name of the release.

You may set a **"version"** field. The value must match the [constraints specification](https://github.com/Masterminds/semver?tab=readme-ov-file#checking-version-constraints) for this library.

You may set a **"github_repository"** field. This should be where the BOSH Release source is maintained. It is used for generating Release Notes for your tile.

You may set a **"float_always"** field. When you set this, `kiln glaze` will not lock this release's version.

You may set a **"maintenance_version_bump_policy"** field. When you run `kiln deglaze` this field specifies how to reset the version constraint after `kiln glaze` locked it.
- "LockNone": Given a glazed version value "1.2.3", this setting resets the version constraint to `"*"`
- "LockPatch": Given a glazed version value "1.2.3", this setting resets the version constraint to `"1.2.3"`
- "LockMinor": Given a glazed version value "1.2.3", this setting resets the version constraint to `"1.2.*"`
- "LockMajor": (default) Given a glazed version value "1.2.3", this setting resets the version constraint to `"1.*"`

You may set a **"slack"** field. Kiln does not use this field. It can be useful for product tile Authors to know who to reach out to when something goes wrong.

#### "bake_configurations"

You may add a list of `kiln bake` flags in the Kilnfile to keep a record of how your tile was baked and to keep CI scripts simpler.

If you set add more than one element to the bake_configurations list, you need to select one by adding a `kiln bake --variables=tile_name=big-footprint-topology` flag corresponding to a bake configuration with a `- tile_name: big-footprint-topology` element

These are the mappings from bake flag to each field in a bake_configurations element:

| bake_configurations element field      | bake flag                      | documentation                                                                         |
|----------------------------------------|--------------------------------|---------------------------------------------------------------------------------------|
| `"tile_name"`                          | `--variables=tile_name=`       | This field is used when selecting a configuration from a list of bake_configurations. |
| `"metadata_filepath"`                  | `--metadata=`                  | This should be the path to the product template entrypoint. Usually called `base.yml` |
| `"icon_filepath"`                      | `--icon=`                      | This may be a path to a png file.                                                     |
| `"forms_directories"`                  | `--forms-directory=`           | This may be a list of directories.                                                    |
| `"instance_groups_directories"`        | `--instance-groups-directory=` | This may be a list of directories.                                                    |
| `"jobs_directories"`                   | `--jobs-directory=`            | This may be a list of directories.                                                    |
| `"migrations_directories"`             | `--migrations-directory=`      | This may be a list of directories.                                                    |
| `"properties_directories"`             | `--properties-directory=`      | This may be a list of directories.                                                    |
| `"runtime_configurations_directories"` | `--runtime-configs-directory=` | This may be a list of directories.                                                    |
| `"bosh_variables_directories"`         | `--bosh-variables-directory=`  | This may be a list of directories.                                                    |
| `"embed_files"`                        | `--embed=`                     | This may be a list of filepaths.                                                      |
| `"variable_files"`                     | `--variables-file=`            | This may be a list of filepaths.                                                      |

### The Lock File [(source)](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg/cargo#Kilnfile)

This file specifies the exact BOSH Release tarballs to package in a tile.

#### `releases`

This is an array of [BOSH Release locks](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg/cargo#BOSHReleaseTarballLock).
Elements will be modified by running `kiln update-release`.
Each element in the releases array in the Kilnfile will have a corresponding element in the Kilnfile.lock releases array.

The release name, release version, sha1 checksum, remote_source, remote_path are fields on each element. 

## Subcommands

### `help`

```
Usage: kiln [options] <command> [<args>]
  --help, -h     bool  prints this usage information (default: false)
  --version, -v  bool  prints the kiln release version (default: false)

Commands:
  bake                     bakes a tile
  cache-compiled-releases  Cache compiled releases
  fetch                    fetches releases
  find-release-version     prints a json string of a remote release satisfying the Kilnfile version and stemcell constraints
  find-stemcell-version    prints the latest stemcell version from Pivnet using the stemcell type listed in the Kilnfile
  generate-osm-manifest    Print an OSM-format manifest.
  glaze                    Pin versions in Kilnfile to match lock.
  help                     prints this usage information
  publish                  publish tile on Pivnet
  re-bake                  re-bake constructs a tile from a bake record
  release-notes            generates release notes from bosh-release release notes
  sync-with-local          update the Kilnfile.lock based on local releases
  test                     Test manifest for a product
  update-release           bumps a release to a new version
  update-stemcell          updates stemcell and release information in Kilnfile.lock
  upload-release           uploads a BOSH release to an s3 release_source
  validate                 validate Kilnfile and Kilnfile.lock
  version                  prints the kiln release version
```

### `bake`

It takes release and stemcell tarballs, metadata YAML, and JavaScript migrations
as inputs and produces an OpsMan-compatible tile as its output.

The produce a tile, you simply need to be within a tile directory and execute the following command:
```
$ kiln bake 
```

This will ensure that you have the necessary releases by first calling `kiln fetch`.

Refer to the [example-tile](example-tile) for a complete example showing the
different features kiln supports.

<details>
  <summary>Additional bake options</summary>

##### `--allow-only-publishable-releases`

The `--allow-only-publishable-releases` flag should be used for development only
and allows additional releases other than those specified in the kilnfile.lock to 
be included in the tile

##### `--bosh-variables-directory`

The `--bosh-variables-directory` flag can be used to include CredHub variable
declarations. You should prefer the use of variables rather than Ops Manager
secrets. Each `.yml` file in the directory should define a top-level `variables`
key.

This flag can be specified multiple times if you have organized your
variables into subdirectories for development convenience.

Example [variables](example-tile/bosh_variables) directory.

##### `--download-threads`

The `--download-threads` flag is for those using S3 as a BOSH release source.
This flag sets the number of parallel threads to download parts from S3

##### `--embed`

The `--embed` flag is for embedding any extra files or directories into the
tile. There are very few reasons a tile developer should want to do this, but if
you do, you can include these extra files here. The flag can be specified
multiple times to embed multiple files or directories.

##### `--final`

The `--final` flag is to bake a final release tile. When passing the --final flag, 
Kiln creates a baked record file with metadata like source revision SHA, tile version, kiln version and 
file checksums. This bake record file will be created under bake_records folder. This 
bake record file can later be used to re-bake the tile. 

##### `--forms-directory`

The `--forms-directory` flag takes a path to a directory that contains one
or more forms. The flag can be specified more than once.

To reference a form file in the directory you can use the `form`
template helper:

```
$ cat /path/to/metadata
---
form_types:
- $( form "first" )
```

Example [forms](example-tile/forms) directory.

##### `--icon`

The `--icon` flag takes a path to an icon file.

To include the base64'd representation of the icon you can use the `icon`
template helper:

```
$ cat /path/to/metadata
---
icon_image: $( icon )
```

##### `--instance-groups-directory`

The `--instance-groups-directory` flag takes a path to a directory that contains one
or more instance groups. The flag can be specified more than once.

To reference an instance group in the directory you can use the `instance_group`
template helper:

```
$ cat /path/to/metadata
---
job_types:
- $( instance_group "my-instance-group" )
```

Example [instance-groups](example-tile/instance_groups) directory.

##### `--jobs-directory`

The `--jobs-directory` flag takes a path to a directory that contains one
or more jobs. The flag can be specified more than once.

To reference a job file in the directory you can use the `job`
template helper:

```
$ cat /path/to/instance-group
---
templates:
- $( job "my-job" )
- $( job "my-aliased-job" )
- $( job "my-errand" )
```

Example [jobs](example-tile/jobs) directory.

You may find that you want to define different job files for the same BOSH job
with different properties. To do this you add an `alias` key to the job which
will be used in preference to the job name when resolving job references:

```
$ cat /path/to/my-aliased-job
---
name: my-job
alias: my-aliased-job
```

##### `--kilnfile`

The `--kilnfile` flag is required with kiln version v0.84.0 and later
The flag expects filepath to a Kilnfile (default: Kilnfile). This
file contain links to all the bosh sources used to build a tile

See the [Kilnfile](#kilnfile) section for more information on Kilnfile formatting



Tile authors will also need to include a Kilnfile.lock in the same directory 
as the Kilnfile. 

See the [Kilnfile.lock](#kilnfile-lock) section for more information on Kilnfile.lock formatting

##### `--metadata`

Specify a file path to a tile metadata file for the `--metadata` flag. This
metadata file will contain the contents of your tile configuration as specified
in the OpsManager tile development documentation.

##### `--metadata-only`

The `--metadata-only` flag outputs the generated metadata to stdout. 
This flag cannot be used with `--output-file`.

##### `--migrations-directory`

If your tile has JavaScript migrations, then you will need to include the
`--migrations-directory` flag. This flag can be specified multiple times if you
have organized your migrations into subdirectories for development convenience.

##### `--no-confirm`

The `no-confirm` flag will delete extra releases in releases directory without prompting.
This flag defaults to `true`

##### `--output-file`

The `--output-file` flag takes a path to the location on the filesystem where
your tile will be created. The flag expects a full file name like
`tiles/my-tile-1.2.3-build.4.pivotal`.

Cannot be used with `--metadata-only`.

##### `--properties-directory`

The `--properties-directory` flag takes a path to a directory that contains one
or more blueprint property files. The flag can be specified more than once.

To reference a properties file in the directory you can use the `property`
template helper:

```
$ cat /path/to/metadata
---
property_blueprints:
- $( property "rep_password" )
```

Example [properties](example-tile/properties) directory.

##### `--releases-directory`

The `--releases-directory` flag takes a path to a directory that contains one or
many release tarballs. The flag can be specified more than once. This is
useful if you consume bosh releases as Concourse resources. Each release will
likely show up in the task as a separate directory. For example:

```
$ tree /path/to/releases
|-- first
|   |-- cflinuxfs2-release-1.166.0.tgz
|   `-- consul-release-190.tgz
`-- second
    `-- nats-release-22.tgz
```

To reference a release you can use the `release` template helper:

```
$ cat /path/to/metadata
---
releases:
- $( release "cflinuxfs2" )
- $( release "consul" )
- $( release "nats" )
```

Example kiln command line:

```
$ kiln bake \
    --version 2.0.0 \
    --metadata /path/to/metadata.yml \
    --releases-directory /path/to/releases/first \
    --releases-directory /path/to/releases/second \
    --stemcells-directory /path/to/stemcells/first \
    --stemcells-directory /path/to/stemcells/second \
    --output-file /path/to/cf-2.0.0-build.4.pivotal
```

##### `--runtime-configs-directory`

The `--runtime-configs-directory` flag takes a path to a directory that
contains one or more runtime config files. The flag can be specified
more than once.

To reference a runtime config in the directory you can use the `runtime_config`
template helper:

```
$ cat /path/to/metadata
---
runtime_configs:
- $( runtime_config "first-runtime-config" )
```

Example [runtime-configs](example-tile/runtime_configs) directory.

##### `--sha256`

The `--sha256` flag calculates the sha256 checksum of the output file

##### `--skip-fetch-directories`

The `--skip-fetch-directories` skips the automatic release fetching of 
the specified release directories


##### `--stemcell-tarball` (Deprecated)

*Warning: `--stemcell-tarball` will be removed in a future version of kiln.
Use `--stemcells-directory` in the future.*

The `--stemcell-tarball` flag takes a path to a stemcell.

To include information about the stemcell in your metadata you can use the
`stemcell` template helper:

```
$ cat /path/to/metadata
---
stemcell_criteria: $( stemcell )
```

##### `--stemcells-directory`

The `--stemcells-directory` flag takes a path to a directory containing one
or more stemcells.

To include information about the stemcell in your metadata you can use the
`stemcell` template helper. It takes a single argument that specifies which
stemcell os.

The `stemcell` helper does not support multiple versions of the same operating
system currently.

```
$ cat /path/to/metadata
---
stemcell_criteria: $( stemcell "ubuntu-xenial" )
additional_stemcells_criteria:
- $( stemcell "windows" )
```

##### `--stub-releases`

For tile developers looking to get some quick feedback about their tile
metadata, the `--stub-releases` flag will skip including the release tarballs
into the built tile output. This should result in a much smaller file that
should upload much more quickly to OpsManager.

##### `--variable`

The `--variable` flag takes a `key=value` argument that allows you to specify
arbitrary variables for use in your metadata. The flag can be specified
more than once.

To reference a variable you can use the `variable` template helper:

```
$ cat /path/to/metadata
---
$( variable "some-variable" )
```

##### `--variables-file`

The `--variables-file` flag takes a path to a YAML file that contains arbitrary
variables for use in your metadata. The flag can be specified more than once.

To reference a variable you can use the `variable` template helper:

```
$ cat /path/to/metadata
---
$( variable "some-variable" )
```

Example [variables file](example-tile/variables.yml).

##### `--version`

The `--version` flag takes the version number you want your tile to become.

To reference the version you use the `version` template helper:

```
$ cat /path/to/metadata
---
product_version: $( version )
provides_product_versions:
- name: example
  version: $( version )
```
</details>

### `re-bake`
It constructs a tile from a given bake record file. 

To run the command, you simply need to be within a tile directory and execute the following command:
```
$ kiln re-bake --output-file tile.pivotal bake_records/1.0.0.json
```

Any variables that Kilnfile needs for the kiln re-bake command should be set in 
~/.kiln/credentials.yml file

### `test`

The `test` command exercises to ginkgo tests under the `/<tile>/test/manifest` and `/<tile>/migrations` paths of the `pivotal/tas` repos (where `<tile>` is tas, ist, or tasw). 

Running these tests require a docker daemon and ssh-agent to be running. 

If you run into this docker error `could not execute "test": failed to connect to Docker daemon: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running`, 
then create a symlink `sudo ln -s ~/.docker/run/docker.sock /var/run/docker.sock`

If no ssh identity is added (check with `ssh-add -l`) , then `kiln test`
will add a ssh key in the following order, prompting for a passphrase if required:
```
	~/.ssh/id_rs
	~/.ssh/id_dsa
	~/.ssh/d_ecdsa
	~/.ssh/d_ed25519
	~/.ssh/dentity
```

The identity must have access to github.com/pivotal/ops-manager.

Here are command line examples:
```
$ cd ~/workspace/tas/ist
$ kiln test
```

```
cd ~
$ kiln test --verbose -tp ~/workspace/tas/ist --ginkgo-manifest-flags "-p -nodes 8 -v" 
```

<details>
  <summary>Additional test options</summary>

##### `--ginkgo-manifest-flags`

The `--ginkgo-manifest-flags` flag can be used to pass through Ginkgo test flags. The defaults being passed through are `-r -p -slowSpecThreshold 15`. Pass `help` as a flag to retrieve the available options for the embeded version of ginkgo.

#### `--manifest-only`

The `--manifest-only` flag can be used to run only Manifest tests. If not passed, `kiln test` will run both Manifest and Migration tests by default.

#### `--migrations-only`
	
The `--migrations-only` flag can be used to run only Migration tests. If not passed, `kiln test` will run both Manifest and Migration tests by default.

##### `--tile-path`

The `--tile-path` (`-tp`) flag can be set the path the directory you wish to test. It defaults to the current working directory. For example
```
$ kiln test -tp ~/workspace/tas/ist
```

##### `--verbose`

The `--verbose` (`-v`) flag will log additional debugging info.

</details>

### `fetch`

The `fetch` command downloads bosh release tarballs specified in the Kilnfile and 
Kilnfile.lock files to a local directory specified by the `--releases-directory` flag. 


Kiln verifies that the checksum (SHA1) of the downloaded release matches
checksum specified for the release in the Kilnfile.lock file. If the checksums do
not match, then the releases that don't match will be deleted from disk. *Since
BOSH releases from different directors with the same packages result in complied
releases with different hashes this may result in some problems where if you
download a release that was compiled with a different director those releases
will be deleted.*

Kiln will not download releases if an existing release exists with the correct
release version and checksum.

<a id="kilnfile"></a>
## Kilnfile
A Kilnfile contains information about the bosh releases and stemcell used by 
a particular tile

Example Kilnfile:
```yaml
---
slug: some-slug #optional but if included should match network.pivotal.io
release_sources:
- type: bosh.io
  releases:
- name: bpm
  version: '*'
stemcell_criteria:
  os: ubuntu-xenial
  version: "~621"
```

#### Supported release sources
##### Bosh.io
  ```yaml
  release_sources:
  - type: bosh.io
  ```
##### s3
```yaml
  release_sources:
  - type: s3
    id: unique-name
    publishable: true # if this bucket contains releases that are suitable to ship to customers
    bucket: some-bucket-in-s3
    region: us-east-1 # must be the region of the above bucket
    access_key_id: $(variable "s3_access_key_id") # Must have at least read permissions to bucket
    secret_access_key: $(variable "s3_secret_access_key")
    path_template: bosh-releases/compiled/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz # See Templating
```

##### github
```yaml
  - type: github
    id: optional-unique-name-defaults-to-github-org-name
    org: the-github-org
    endpoint: $(variable "github_host")
    github_token: $(variable "github_token")
```

##### artifactory
```yaml
  - type: artifactory
    id: unique-name
    artifactory_host: https://build-artifactory.your-artifactory-url.com
    repo: some-artifactory-repo 
    publishable: true # if this repo contains releases that are suitable to ship to customers
    username: $(variable "artifactory_username")
    password: $(variable "artifactory_password")
    path_template: shared-releases/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz # See Templating
```
<a id="kilnfile-templating"></a>
### Templating
#### Options
Kilnfile files support the following templating options:

- `{{.Name}}` for release name 
- `{{.Version}}` for release version 
- `{{.StemcellOS}}` for stemcell OS 
- `{{.StemcellVersion}}` for stemcell version 

- There's also access to a `trimSuffix` helper (e.g. `{{trimSuffix .Name "-release"}}`)

#### Functions
##### `select`

The `select` function allows you to pluck values for nested fields from a
template helper.

For instance, this section in our example tile:

```yaml
my_release_version: $( release "my-release" | select "version" )
```

Results in:

```yaml
my_release_version: 1.2.3
```

#### Variable Interpolation

```yaml
release_sources:
  - type: s3
    compiled: true
    bucket: compiled-releases
    region: us-west-1
    access_key_id: $(variable "aws_access_key_id")
    secret_access_key: $(variable "aws_secret_access_key")
    path_template: 2.6/{{trimSuffix .Name "-release"}}/{{.Name}}-{{.Version}}-{{.StemcellOS}}-{{.StemcellVersion}}.tgz
```

*Credentials like S3 keys are not stored in git repos. To support separating
that information from non-sensitive configuration, you can reference variables
like you do in tile config.*

```yaml
---
aws_access_key_id: SOME_REALLY_SECRET_ID
aws_secret_access_key: SOME_REALLY_SECRET_KEY
```

Interpolating this file in kiln would look something like this.

1. Get your credentials from Lastpass by running: `lpass show --notes 'pas-releng-fetch-releases' > vars.yml`

```bash
kiln bake --kilnfile random-Kilnfile --variables-file vars.yml
```

<a id="kilnfile-lock"></a>
### Kilnfile.lock

The Kilnfile.lock file name is expected to be a file in the same directory as the
Kilnfile with `lock` as as the filename extension.

This file contains the full list of specific versions of all releases, shas, and sources for 
bosh releases that will go into the tile as well as the target stemcell.

The file has two top level members `releases` and `stemcell_criteria`.

The `releases` member is an array of members with each element having the following members.
- `name`: bosh release name
- `sha1`: checksum of the tarball
- `version`: semantic version of the release
- `remote_source`: the resource-type for bosh.io or the id for the other types
- `remote_path`: the path that where the bosh release is stored

The `stemcell_criteria ` member is defines the stemcell used to generate the tile
- `os`: the stemcell os used (e.g. ubuntu-xenial)
- `version`: semantic version of the stemcell

Example Kilnfile.lock :
```yaml
releases:
- name: bpm
  sha1: 86675f90d66f7018c57f4ae0312f1b3834dd58c9
  version: 1.1.18
  remote_source: bosh.io
  remote_path: https://bosh.io/d/github.com/cloudfoundry/bpm-release?v=1.1.18
- name: backup-and-restore-sdk
  sha1: 0f48faa2f85297043e5201e2200567c2fe5a9f9a
  version: 1.18.84
  remote_source: unique-name # this could be artifactory or s3
  remote_path: bosh-releases/compiled/backup-and-restore-sdk-1.18.84-ubuntu-jammy-1.179.tgz
- name: hello-release
  sha1: 06500a2002f6e14f6c258b7ee7044761a28d3d5a
  version: 0.1.5
  remote_source: the-github-org 
  remote_path: https://github.com/crhntr/hello-release/releases/download/v0.1.5/hello-release-v0.1.5.tgz
stemcell_criteria:
  os: ubuntu-xenial
  version: "621.0"
```

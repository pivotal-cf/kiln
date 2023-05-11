# Kiln [![release](https://github.com/pivotal-cf/kiln/actions/workflows/release.yml/badge.svg)](https://github.com/pivotal-cf/kiln/actions/workflows/release.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/pivotal-cf/kiln.svg)](https://pkg.go.dev/github.com/pivotal-cf/kiln/pkg)

_Kiln bakes tiles_

Kiln helps tile developers build products for [VMware Tanzu Operations Manager](https://network.tanzu.vmware.com/products/ops-manager/). It provides
an opinionated folder structure and templating capabilities. It is designed to be used
both in CI environments and in command-line to produce a tile.

## Instalation

To install the `kiln` CLI 
- install with Homebrew

  ```sh
  brew tap pivotal-cf/kiln https://github.com/pivotal-cf/kiln
  brew install kiln
  ```

- download from the [releases page](https://github.com/pivotal-cf/kiln/releases)

  ```sh
  export KILN_VERSION
  KILN_VERSION="$(curl -H "Accept: application/vnd.github.v3+json" 'https://api.github.com/repos/pivotal-cf/kiln/releases?per_page=1' | jq -r '.[0].name')"
  curl -L -o kiln "https://github.com/pivotal-cf/kiln/releases/download/${KILN_VERSION}/kiln-darwin-${KILN_VERSION}"
  # check the checksum
  cp kiln "$(go env GOPATH)/bin"
  kiln version
  ```

- build from source
  
  ```sh
  git clone git@github.com:pivotal-cf/kiln.git
  cd kiln
  git checkout 0.60.2
  ./install.sh
  ```

- copy from a Docker image (to another image)

  ```sh
  docker pull pivotalcfreleng/kiln:latest
  ```

   ```Dockerfile
  FROM pivotalcfreleng/kiln:latest as kiln

  FROM ubuntu
  COPY --from=kiln /kiln /usr/bin/kiln
  CMD /usr/bin/bash
   ```

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

##### `--bosh-variables-directory`

The `--bosh-variables-directory` flag can be used to include CredHub variable
declarations. You should prefer the use of variables rather than Ops Manager
secrets. Each `.yml` file in the directory should define a top-level `variables`
key.

This flag can be specified multiple times if you have organized your
variables into subdirectories for development convenience.

Example [variables](example-tile/bosh_variables) directory.

##### `--embed`

The `--embed` flag is for embedding any extra files or directories into the
tile. There are very few reasons a tile developer should want to do this, but if
you do, you can include these extra files here. The flag can be specified
multiple times to embed multiple files or directories.

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

##### `--metadata`

Specify a file path to a tile metadata file for the `--metadata` flag. This
metadata file will contain the contents of your tile configuration as specified
in the OpsManager tile development documentation.

##### `--metadata-only`

Output the generated metadata to stdout. Cannot be used with `--output-file`.

##### `--migrations-directory`

If your tile has JavaScript migrations, then you will need to include the
`--migrations-directory` flag. This flag can be specified multiple times if you
have organized your migrations into subdirectories for development convenience.

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

##### `--stemcells-directory`

The `--stemcell-directory` flag takes a path to a directory containing one
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

#### `--skip-fetch-directories`

The `--skip-fetch-directories` flag bypasses the default behavior to fetch releases when running `kiln bake`.

</details>


### `test`

The `test` command exercises to ginkgo tests under the `/<tile>/test/manifest` path of the `pivotal/tas` repos (where `<tile>` is tas, ist, or tasw). 

Running these tests require a docker daemon and ssh-agent to be running. If no ssh identity is added (check with `ssh-add -l`) , then `kiln test`
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

##### `--tile-path`

The `--tile-path` (`-tp`) flag can be set the path the directory you wish to test. It defaults to the current working directory. For example
```
$ kiln test -tp ~/workspace/tas/ist
```

##### `--verbose`

The `--verbose` (`-v`) flag will log additional debugging info.

</details>

### `fetch`

The `fetch` command downloads bosh release tarballs from an AWS S3 bucket to a
a local directory specified by the `--releases-directory` flag. It discovers
releases based on information from both the Kilnfile and an Kilnfile.lock file.
The Kilnfile.lock file name is expected to be a file in the same directory as the
specified Kilnfile with `lock` as as the filename extension.
The S3 object name is determined based on using regular expression capture
groups.

Kiln verifies that the checksum (SHA1) of the downloaded release matches
checksum specified for the release in the Kilnfile.lock file. If the checksums do
not match, then the releases that don't match will be deleted from disk. *Since
BOSH releases from different directors with the same packages result in complied
releases with different hashes this may result in some problems where if you
download a release that was compiled with a different director those releases
will be deleted.*

Kiln will not download releases if an existing release exists with the correct
release version and checksum.

#### Kilnfile
The Kilnfile must also have information about how to access the S3 Bucket.
Two types of release sources are allowed in the list under the `release_sources`
key:

1. `type: bosh.io`. For this type, no other keys are required/allowed.
2. `type: s3`. The following other keys **required** in this case.

- `publishable` (boolean): true if this bucket contains releases that are suitable to ship to customers
- `bucket`: must be the name of the s3 bucket
- `region`: must be the region of the bucket
- `access_key_id`: must be an IAM access key id that has read permission for the
  specified bucket
- `secret_access_key`: must be the secret for the specified `access_key_id`
- `release_path:`: a (text/template package) template expression used to build the 
  full-path to a release in the S3 bucket. The template should evaluate to the exact 
  path within the s3 bucket for a given release name+version+stemcell combination. 
  The template has access to the following fields:
  - release name (e.g. `{{.Name}}`)
  - release version (e.g. `{{.Version}}`)
  - stemcell OS (e.g. `{{.StemcellOS}}`)
  - stemcell version (e.g. `{{.StemcellVersion}}`)
  - There's also access to a `trimSuffix` helper (e.g. `{{trimSuffix .Name "-release"}}`)

### Kilnfile.lock

This file contains the full list of specific versions of all releases that will
go into the tile AND the target stemcell.

Currently the releases for the Kilnfile.lock file can not be generated by kiln.
The update command is in development and only (loosely) supports updating the
stemcell based on stemcells on https://network.pivotal.io. On PAS Release
Engineering we use a consourse task in our CI to generate the Kilnfile.lock
file.

The file has two top level members `releases` and `stemcell_criteria`.

The `releases` member is an array of members with each element having the following members.
- `name`: bosh release name
- `sha1`: checksum of the tarball
- `version`: semantic version of the release

The `stemcell_criteria ` member is an array of members with each element having the following members.
- `name`: bosh release name
- `sha1`: checksum of the tarball
- `version`: semantic version of the release

### Example with Variable Interpolation

```
$ cat Kilnfile
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

```
$ lpass show --notes 'pas-releng-fetch-releases'
---
aws_access_key_id: SOME_REALLY_SECRET_ID
aws_secret_access_key: SOME_REALLY_SECRET_KEY
```

Interpolating this file in kiln would look something like this.

```
kiln fetch --kilnfile random-Kilnfile --variables-file <(lpass show --notes 'pas-releng-fetch-releases')
```

### Template functions

#### `select`

The `select` function allows you to pluck values for nested fields from a
template helper.

For instance, this section in our example tile:

```
my_release_version: $( release "my-release" | select "version" )
```

Results in:

```
my_release_version: 1.2.3
```

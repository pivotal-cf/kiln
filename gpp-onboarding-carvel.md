# Onboarding Carvel/Kubernetes Tiles to Golden Path to Publish

This playbook walks Kubernetes tile teams through onboarding their tiles onto the Golden Path to Publish (GPP) workflow using **`kiln carvel`** commands.

> [!NOTE]
> For Carvel-based tiles, intermediary BOSH releases are generated automatically from your imgpkg bundle. You do **not** need to manage BOSH releases directly -- Kiln handles that behind the scenes.

The result of this work will give you a **re-bakable tile** in [Artifactory](https://usw1.packages.broadcom.com/ui/repos/tree/General/tas-ecosystem-generic-prod-local/tile-releases) that is scanned by BlackDuck. You may also optionally configure RMT releases and Open Source License Disclosure files.

---

## Tile directory structure

Your Kubernetes tile repository must contain the following structure:

```
my-tile/
├── base.yml                   # Tile metadata (name, version, package_installs, etc.)
├── bundle.tar                 # imgpkg bundle containing Carvel packages
├── version                    # Tile version (e.g. "1.0.0")
├── Kilnfile                   # Artifactory release source config (for upload/publish/rebake)
├── packageinstalls/           # Package install definitions
│   └── <name>.yml
├── properties/                # Property blueprints (optional)
│   └── *.yml
├── forms/                     # Form definitions (optional)
│   └── *.yml
├── icon.png                   # Tile icon (optional but recommended)
└── .gitignore                 # Should ignore .boshrelease/ and .carvel-tile/
```

> [!IMPORTANT]
> - `base.yml` must have `metadata_version >= 3.2.0` (required for Kubernetes tile support).
> - `base.yml` must include a `package_installs` array and a `compatible_kubernetes_distributions` array.

---

## Prerequisites

### Required tools

| Tool | Purpose | Install |
|------|---------|---------|
| **Kiln** | Tile building and publishing | [github.com/pivotal-cf/kiln](https://github.com/pivotal-cf/kiln) |
| **BOSH CLI** | BOSH release generation from imgpkg bundle | [bosh.io/docs/cli-v2-install](https://bosh.io/docs/cli-v2-install/) |

### GitHub repo access

Provide **write** access to your tile repository to our bot account. Since BOSH releases are generated from your tile source, separate BOSH release repos are not required.

- GitHub Enterprise (`github.gwd.broadcom.com`): [tanzu-tas-ecosystem](https://github.gwd.broadcom.net/tanzu-tas-ecosystem)
- GitHub.com: [tas-ecosystem-bot](https://github.com/tas-ecosystem-bot)

### TNZ team membership

To create PRs against the configuration repo, you need to be a member of the [`all`](https://github.gwd.broadcom.net/orgs/TNZ/teams/all) team in the [TNZ org](https://github.gwd.broadcom.net/orgs/TNZ).

---

## Artifactory access and credentials

### Getting access

Authentication is required for Broadcom JFrog Artifactory. Create a [Support Ticket](https://broadcomitsm.wolkenservicedesk.com/wolken-support/item_details?itemId=2422) with:

- **Artifactory Server Name / URL**: `https://usw1.packages.broadcom.com/ui`
- **Business Justification**:

  ```text
  Need read/write access to tas-ecosystem-* artifactory projects
  on https://usw1.packages.broadcom.com

  For the following teammates / service accounts:
  - memberX
  - memberY
  - bot / service account
  ```

### Creating an API key or identity token

Since Artifactory uses Okta SSO, password authentication is not available. You need an `api_key` or `identity_token`:

1. Log in to the [Artifactory UI](https://usw1.packages.broadcom.com/ui) via SSO.
2. Click the dropdown **Welcome, your_username** in the upper right.
3. Click **Edit Profile**.
4. Create an `api_key` or `identity_token` -- this value is used as the password for all `kiln` commands.

> [!WARNING]
> `usw1.packages.broadcom.com` is only accessible on the Broadcom network. If accessing remotely, **full tunnel VPN is required**.
>
> If your CI is on the VMware / Broadcom network and is blocked, reach out to [#VMW-harbor-jfrog-migration](https://chat.google.com/room/AAAAcWIWWOA?cls=7).

---

## Providing credentials to Kiln

The `kiln carvel` commands that interact with Artifactory (`upload`, `publish`, `rebake`, and `bake` with a Kilnfile.lock) need credentials. These are configured in the **Kilnfile** using variable interpolation and resolved at runtime.

### Step 1: Set up your Kilnfile

Create a `Kilnfile` in your tile directory with variable placeholders:

```yaml
release_sources:
  - type: artifactory
    artifactory_host: $( variable "artifactory_host" )
    repo: $( variable "artifactory_repo" )
    username: $( variable "artifactory_username" )
    password: $( variable "artifactory_password" )
    path_template: "bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz"
```

### Step 2: Choose how to provide the variable values

There are three ways to supply credentials, in order of precedence (highest first):

#### Option A: `--variable` flags (best for CI)

Pass each value directly on the command line:

```bash
kiln carvel upload \
  --source-directory . \
  --variable artifactory_host=https://usw1.packages.broadcom.com \
  --variable artifactory_repo=tas-ecosystem-generic-prod-local \
  --variable artifactory_username=my-bot-account \
  --variable artifactory_password=cmVmdGtuOj...
```

> [!TIP]
> Use `-vr` as the short form for `--variable`.

#### Option B: `--variables-file` flag

Point to a YAML file containing the values:

```bash
kiln carvel upload \
  --source-directory . \
  --variables-file path/to/credentials.yml
```

Where `credentials.yml` contains:

```yaml
artifactory_host: https://usw1.packages.broadcom.com
artifactory_repo: tas-ecosystem-generic-prod-local
artifactory_username: my-bot-account
artifactory_password: cmVmdGtuOj...
```

> [!TIP]
> Use `-vf` as the short form for `--variables-file`.

#### Option C: `~/.kiln/credentials.yml` (best for local development)

When the internal `kiln bake` step runs (inside `upload`, `publish`, `rebake`, and `bake`), Kiln automatically loads `~/.kiln/credentials.yml` as a default variables file. Place your credentials there for a seamless local experience:

```yaml
# ~/.kiln/credentials.yml
artifactory_host: https://usw1.packages.broadcom.com
artifactory_repo: tas-ecosystem-generic-prod-local
artifactory_username: your_username
artifactory_password: your_api_key_or_identity_token
```

> [!IMPORTANT]
> The `~/.kiln/credentials.yml` auto-loading only applies to the internal `kiln bake` step. The outer `kiln carvel` commands (which parse the Kilnfile for Artifactory config) still need credentials via `--variable` or `--variables-file` -- **unless** you hardcode the values directly in the Kilnfile (not recommended for secrets).
>
> For the simplest local workflow, use both: put credentials in `~/.kiln/credentials.yml` **and** pass `--variables-file ~/.kiln/credentials.yml` to the outer command.

> [!CAUTION]
> Never commit credentials to your repository. Add `credentials.yml` and `~/.kiln/` to your `.gitignore`.

### Artifactory variable values

| Variable | Value |
|----------|-------|
| `artifactory_host` | `https://usw1.packages.broadcom.com` |
| `artifactory_repo` | `tas-ecosystem-generic-prod-local` |
| `artifactory_username` | Your account or service account |
| `artifactory_password` | Your `api_key` or `identity_token` |

---

## Developer workflow

The `kiln carvel` commands cover the full tile development lifecycle. Each step builds on the previous one:

```
  Local dev             CI integration            Final release           CI publish
┌──────────────┐   ┌──────────────────┐   ┌──────────────────────┐   ┌──────────────────┐
│ carvel bake  │──▶│  carvel upload   │──▶│ carvel publish       │──▶│  carvel rebake   │
│ (local only) │   │ (uploads + lock) │   │ --final (bake record)│   │ (reproducible)   │
└──────────────┘   └──────────────────┘   └──────────────────────┘   └──────────────────┘
                          │                          │                         │
                    git commit              git commit bake record       checksum verified
                    Kilnfile.lock                                       .pivotal uploaded
```

### Step 1: Local bake (no Artifactory needed)

Bake a tile locally to test your tile structure. No Kilnfile or credentials required.

```bash
kiln carvel bake \
  --source-directory . \
  --output-file my-tile-0.1.0.pivotal
```

This generates a BOSH release from your `bundle.tar`, assembles the tile, and produces a `.pivotal` file. Nothing is uploaded; no lockfile is created.

### Step 2: Upload to Artifactory

Once your local bake works, upload the generated BOSH release to Artifactory so CI can reuse it:

```bash
kiln carvel upload \
  --source-directory . \
  --variables-file ~/.kiln/credentials.yml
```

This command:

1. Generates a BOSH release from your imgpkg bundle.
2. Uploads the tarball to Artifactory using the Kilnfile's release source config.
3. Writes a `Kilnfile.lock` with the release name, version, SHA1, and remote path.

Then commit the lockfile:

```bash
git add Kilnfile.lock
git commit -m "Add Kilnfile.lock from carvel upload"
```

> [!NOTE]
> You can also pass `--output-file my-tile.pivotal` to upload to bake a `.pivotal` in the same step.

### Step 3: CI bake (automatic, via Kilnfile.lock)

When Kilnfile.lock is present, `kiln carvel bake` downloads the cached BOSH release from Artifactory instead of regenerating it locally. This is faster and reproducible.

```bash
kiln carvel bake \
  --source-directory . \
  --output-file my-tile-dev.pivotal \
  --variables-file ~/.kiln/credentials.yml
```

This is what your CI pipeline should run for development/candidate tile builds.

### Step 4: Publish a final release

Create a final, versioned tile with a bake record for reproducible builds:

```bash
kiln carvel publish --final \
  --source-directory . \
  --output-file my-tile-1.0.0.pivotal \
  --variables-file ~/.kiln/credentials.yml
```

This command:

1. Downloads the BOSH release from Artifactory (using the Kilnfile.lock).
2. Bakes the tile.
3. Computes a SHA-256 checksum of the `.pivotal` file.
4. Writes a bake record to `bake_records/<version>.json` containing the source revision, version, and file checksum.

Then commit the bake record:

```bash
git add bake_records/
git commit -m "Release version 1.0.0"
```

> [!TIP]
> Use `--version` to override the tile version from the `version` file. For example, `--version 2.1.41` produces `bake_records/2.1.41.json`.

**Example bake record** (`bake_records/1.0.0.json`):

```json
{
  "source_revision": "1b19d8cb80e6cfdddd7be1c7a26c8210cbd4e4c5",
  "version": "1.0.0",
  "kiln_version": "0.97.0",
  "file_checksum": "7622143c54dc53087a6c2401f5030170515e14f466857564a980092d4c87a094",
  "tile_directory": "."
}
```

> [!WARNING]
> Your `bake_records/` directory must **only** contain bake record JSON files.

> [!NOTE]
> Pre-release versions (e.g. `2.4.41-dev.0`) will not trigger a publish. This is useful for verifying OSL triage status before creating a final version like `2.4.41`.

### Step 5: Rebake (CI, automated by GPP)

GPP automatically runs rebake when it detects a new bake record. The rebake command reproduces the tile from the bake record and verifies the checksum matches:

```bash
kiln carvel rebake \
  --output-file my-tile-1.0.0.pivotal \
  --variables-file ~/.kiln/credentials.yml \
  bake_records/1.0.0.json
```

> [!IMPORTANT]
> The repository must be checked out at the **exact commit** recorded in `source_revision`. The rebake will fail if HEAD does not match. In CI, the Concourse resource handles this automatically.

The rebake:

1. Reads the bake record to determine the source revision, version, and expected checksum.
2. Downloads the BOSH release from Artifactory (if Kilnfile.lock is present).
3. Bakes the tile.
4. Verifies the output checksum matches the bake record -- **byte-for-byte reproducibility**.

---

## Command reference

| Command | Description | Requires Kilnfile? | Requires Kilnfile.lock? | Writes Kilnfile.lock? |
|---------|-------------|:-------------------:|:-----------------------:|:---------------------:|
| `kiln carvel bake` | Local bake from bundle | No | No (uses if present) | No |
| `kiln carvel upload` | Upload BOSH release to Artifactory | **Yes** | No | **Yes** |
| `kiln carvel publish --final` | Bake + create bake record | **Yes** | **Yes** | No |
| `kiln carvel rebake <record>` | Reproduce tile from bake record | **Yes** (if lock present) | **Yes** (if present) | No |

### Common flags

| Flag | Short | Description | Used by |
|------|-------|-------------|---------|
| `--source-directory` | `-s` | Path to tile source directory (defaults to `.`) | `bake`, `upload`, `publish` |
| `--output-file` | `-o` | Path for the output `.pivotal` file | `bake`, `upload` (optional), `publish`, `rebake` |
| `--variable` | `-vr` | Key-value pair for Kilnfile interpolation | All commands |
| `--variables-file` | `-vf` | Path to YAML file with variable values | All commands |
| `--kilnfile` | `-kf` | Path to Kilnfile (default: `Kilnfile` in source dir) | All commands |
| `--verbose` | `-v` | Enable verbose output | All commands |
| `--final` | | Create a bake record | `publish` only |
| `--version` | | Override tile version | `publish` only |

---

## Golden Path configuration

Configuration for the TAS Golden Path is stored in the [tas-ecosystem-configuration](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration) repo and used as inputs to generate Concourse pipelines.

> [!NOTE]
> For Carvel-based tiles, you do **not** need to add BOSH release config files to the `bosh/` folder. GPP manages BOSH release ingest and compilation automatically.

### Tile config onboard

1. Clone the [tas-ecosystem-configuration](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration) repo and create a branch.

2. Create a new file for your tile under the `tiles/` directory.

   > [!IMPORTANT]
   > `artifact_name` determines file name prefixes in Artifactory, project name prefixes in BlackDuck, and the prefix for published releases in RMT.

   **Example** -- `my-k8s-tile.yml`:

   ```yaml
   #@data/values
   ---
   repo: https://github.gwd.broadcom.net/TNZ/my-k8s-tile.git
   branch: main
   update_branch: auto-bump
   subpath: .
   artifact_name: my-k8s-tile
   prerelease_format: build_increment_sha
   team_members:
   - alice@broadcom.com
   - bob@broadcom.com
   team_google_chat_group: my-team-chat
   team_slack_channel: my-team-slack     #! optional
   ```

3. **(Optional)** Add fields for automatic RMT draft-release creation.

   > [!WARNING]
   > You will need to add upgrade specifiers (else Upgrade Planner will break!) and verify the release is ready for GA. It defaults to a draft.

   See [tile RMT release](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration/tree/main/docs/tile_rmt_release.md) for details.

4. **(Optional)** Add BlackDuck tile project associations.

   Confirm your project exists at https://broadcom-vmw.app.blackduck.com/ with the format `TNZ-CF-<artifact_name>-tile`. If not, submit a ticket via the [BlackDuck Onboarding section](./creating_open_source_license_disclosures.md#blackduck-onboarding).

   Scanning is enabled by default. To disable:

   ```yaml
   blackduck:
     enabled: false
   ```

5. Create a PR. An automated check will verify your config. On merge, GPP jobs will be created for your tile.

   Reach out to [#tas-slingshots on Google Chat](https://chat.google.com/room/AAAAZuDvKe0?cls=7) with questions.

### Optional: auto-bump branch

Create an `update_branch` (e.g. `autobump`) for GPP to push Kilnfile.lock updates for your review. If `branch` and `update_branch` are the same, force push is disabled. Ensure the bot account has [push access](#github-repo-access) to the specified branch.

---

## End-to-end example

Here is the complete workflow from first local bake to published tile:

```bash
# 1. Local bake -- verify your tile structure works
kiln carvel bake -s . -o my-tile-dev.pivotal

# 2. Upload BOSH release to Artifactory, write lockfile
kiln carvel upload -s . -vf ~/.kiln/credentials.yml

# 3. Commit the lockfile
git add Kilnfile.lock
git commit -m "Add Kilnfile.lock"

# 4. CI bake (downloads cached release from Artifactory)
kiln carvel bake -s . -o my-tile-dev.pivotal -vf ~/.kiln/credentials.yml

# 5. Final release with bake record
kiln carvel publish --final -s . -o my-tile-1.0.0.pivotal -vf ~/.kiln/credentials.yml

# 6. Commit the bake record
git add bake_records/
git commit -m "Release 1.0.0"
git push

# 7. GPP automatically runs rebake, verifies checksum, and publishes to Artifactory
```

---

## Troubleshooting

### `Kilnfile not found`

The `upload`, `publish`, and `rebake` commands require a `Kilnfile` with an `artifactory` release source. Make sure the file exists in your tile's source directory (or pass `--kilnfile path/to/Kilnfile`).

### `Kilnfile.lock not found` or `no releases`

Run `kiln carvel upload` first to generate the BOSH release and create the lockfile.

### `source revision mismatch` during rebake

The repo must be at the exact commit from the bake record's `source_revision`. Check out that commit before running rebake.

### `upload failed with status 401`

Your Artifactory credentials are incorrect or expired. Regenerate your API key or identity token from the [Artifactory UI](https://usw1.packages.broadcom.com/ui).

### `tile checksum mismatch` during rebake

The tile produced by rebake does not match the original publish. This can happen if the source tree has been modified after the bake record was created. Ensure no uncommitted changes exist and that HEAD matches `source_revision`.

### Network errors connecting to Artifactory

`usw1.packages.broadcom.com` requires Broadcom full-tunnel VPN. Verify you are connected.

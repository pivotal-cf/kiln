This playbook provides instructions for Carvel/Kubernetes tile teams to onboard their tiles onto the Golden Path to publish workflow.

For Carvel-based tiles, intermediary BOSH releases are generated automatically from imgpkg bundles (via ezbake) and do not need to be managed directly by your team. GPP handles BOSH release ingest and compilation behind the scenes. Your team only needs to manage final tile releases.

The result of this work will give you a re-bakable tile in [Artifactory](https://usw1.packages.broadcom.com/ui/repos/tree/General/tas-ecosystem-generic-prod-local/tile-releases) with compiled BOSH releases that is scanned by BlackDuck. For TVS integration please notify the Slingshots team when you're ready for it along with a link to your config please.

You may also optionally configure your Tile to generate RMT releases, and Open Source License Disclosure files from Blackduck.

## Pre-requisites for onboarding

### Your tile is built with Kiln

The Golden Path does not currently support tiles built with [tile-generator](https://github.com/cf-platform-eng/tile-generator).

Please consider using [kiln](https://github.com/pivotal-cf/kiln/blob/main/TILE_AUTHOR_GUIDE.md). Carvel tile workflows use the `kiln carvel` subcommand group (`bake`, `upload`, `publish`, `rebake`).

### Github repo access: tiles

Please provide write access to your private tile repositories to our bot account. Because BOSH releases are generated from the tile source (imgpkg bundles), separate BOSH release repositories are not required.

- For github enterprise (github.gwd.broadcom.com): [tanzu-tas-ecosystem](https://github.gwd.broadcom.net/tanzu-tas-ecosystem)
- for github.com: [tas-ecosystem-bot](https://github.com/tas-ecosystem-bot)

### TNZ team membership

To create PRs against our configuration repo you need to be a member of the [`all`](https://github.gwd.broadcom.net/orgs/TNZ/teams/all) team in the [TNZ org](https://github.gwd.broadcom.net/orgs/TNZ).

### Broadcom artifactory access

Authentication is required for accessing repos and artifacts on the Broadcom Jfrog Artifactory service.  To get access for your team to our artifact repos containing: bosh-releases, compiled-releases, tile-releases and tile-candidates, create a  [1.Support Ticket](https://broadcomitsm.wolkenservicedesk.com/wolken-support/item_details?itemId=2422).  
Specify:

- Artifactory Server Name / URL:  `https://usw1.packages.broadcom.com/ui`
- Sample Business Justification:

  ```text
  Need read access to tas-ecosystem-* artifactory projects on https://usw1.packages.broadcom.com

  For the following teammates / service accounts:
  - memberX
  - memberY
  - memberZ
  - bot / service account
  ```

#### Credentials

Since artifactory is authenticated with Okta SSO, password authentication to the service it not allowed.  Artifactory have `api_keys` and `identity_tokens` that are used as passwords.

Once access is granted and you are able to login to the artifactory ui via SSO, an `api_key` or `identity token` needs to be created for use with Kiln

1. Upper right click dropdown of: `Welcome, your_username`
2. Click `Edit Profile`
3. Create an `api_key` or `identity_token` here and use it as the password for `kiln` commands or the artifactory cli.

#### Network access

The `usw1.packages.broadcom.com` artifactory is also only available on the Broadcom network. If accessing remotely, full tunnel VPN is required.

If you CI is on the VMware / Broadcom Network and is blocked from accessing the artifactory, reach out to Google Chat Space: [#VMW-harbor-jfrog-migration](https://chat.google.com/room/AAAAcWIWWOA?cls=7)

## Golden Path Configuration

Configuration for the TAS Golden Path is stored in this repo and used as inputs to generate concourse pipelines.

For Carvel-based tiles, the onboarding is simpler than for traditional BOSH tiles because GPP manages BOSH release ingest and compilation behind the scenes. You do not need to add BOSH release config files to the `bosh/` folder. The existing [bosh-ingest](https://tpe-concourse-rock.acc.broadcom.net/teams/tas-ecosystem/pipelines/bosh-releases?group=ingest-releases) and [bosh-compile](https://tpe-concourse-rock.acc.broadcom.net/teams/tas-ecosystem/pipelines/bosh-releases?group=compile-releases) pipelines are available for inspection if needed but do not require configuration from your team.

Overall the following steps to complete are:

- Updating the `Kilnfile` in the git repository to use artifactory as a source for generated BOSH releases.
- (optional) Creating a branch in the git repository of your tile for a pipeline to push Kilnfile.lock updates for your review.
- Creating config files for your tile(s) in `tiles/` folder to generate pipeline that will:
  - Bake tile candidates with `kiln carvel bake`. Dev builds of tiles on your main / feature branch
  - [`kiln carvel rebake`](https://github.com/pivotal-cf/kiln) for versioned release tiles
  - Associate the BOSH releases consumed by the tile to the Blackduck tile project
  - (optional) Automatically creates RMT releases that are included in the next-available TPM managed Release Train to assist with publishing
    - If RMT is enabled, then your RMT release is eligible for automatic Open Source License Notice inclusion. Please see [creating open source license disclosures](./creating_open_source_license_disclosures.md).
  - (optional) TVS integration (notify the #tas-slingshots team with your tile config requesting this when ready)

### Tile repository updates

Set up your tile repository so that `kiln carvel` commands can fetch generated BOSH releases from Artifactory.

1. In the main / feature branch, update the `Kilnfile` to include artifactory as the remote source for BOSH releases.

   ```yaml
   release_sources:
   - type: artifactory
     id: artifactory_bosh_releases
     artifactory_host: $(variable "artifactory_host")
     repo: $(variable "artifactory_repo")
     username: $(variable "artifactory_username")
     password: $(variable "artifactory_password") # api_key or identity token
     publishable: true # if this repo contains releases that are suitable to ship to customers
     path_template: bosh-releases/{{.Name}}/{{.Name}}-{{.Version}}.tgz
   ```

2. Use `kiln carvel upload` to generate the BOSH release from your imgpkg bundle, upload it to Artifactory, and update the `Kilnfile.lock` with the remote location and checksum.

   Example `kiln carvel upload` command:

   ```bash
   $ kiln carvel upload \
      --artifactory-host https://usw1.packages.broadcom.com \
      --artifactory-repo tas-ecosystem-generic-prod-local \
      --artifactory-username <your_user_or_bot_account> \
      --artifactory-password <api_key-or-identity_token> \
      --output-file my-tile-1.0.0-dev.pivotal
   ```

   This uploads the generated BOSH release and writes a `Kilnfile.lock` referencing the remote artifact. Commit the updated `Kilnfile.lock` to your repository.

3. (Optional) Create a new update branch from the main / feature branch in your tile repository (eg: `autobump`). Our CI will force push commits to this branch.  
   While this provides an auto update functionality, you are welcome to continue using your existing auto update tools (eg: dependabot).  
   You can also specify your feature branch if you want our CI to push the `Kilnfile.lock` updates directly to your feature branch.  
   If `branch` and `update_branch` are same, force push functionality is disabled. Ensure the branch specified in `update_branch` has [push access to our bot account](#github-repo-access-tiles).

### Tile config onboard

1. Clone [this](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration) git repository if you have not already and create a branch locally for your changes. You should have write access to the repo and not need to create a fork to create a PR. If not, please review [this pre-requisite](#tnz-team-membership)

2. Create a new file for each of your tiles under the [tiles](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration/tree/main/tiles) directory. Please note that `artifact_name` is especially significant because it determines the file name prefix in `artifactory`, project names prefixes in `blackduck`, and is the prefix used by the published release file in RMT / Broadcom Portal (when enabled).   
     Hello Tile - `hello-tile.yml`

     ```yaml
     #@data/values
     ---
     repo: https://github.gwd.broadcom.net/TNZ/hello-tile.git
     branch: main
     update_branch: auto-bump
     subpath: .
     artifact_name: crhntr-hello  #! this is the prefix for the built tiles and must be consistent with blackduck too
     prerelease_format: build_increment_sha #! "sha" or "build_increment_sha" for versioning tile candidate builds. we recommend build_increment_sha
     team_members:
     - a@vmware.com
     - b@vmware.com
     team_google_chat_group: some-group #! required - google space / chat group for your team
     team_slack_channel: some-channel   #! If your team has slack channel
     ```

    Scheduler Tile - `p-scheduler.yml`  (auto update directly on feature branch)

     ```yaml
     #@data/values
     ---
     repo: https://github.com/pivotal-cf/p-scheduler.git
     branch: master
     update_branch: master
     subpath: .
     artifact_name: p-scheduler  #! this is the prefix for the built tiles and must be consistent with blackduck too
     prerelease_format: build_increment_sha #! "sha" or "build_increment_sha" for versioning tile candidate builds
     team_members:
     - a@vmware.com
     - b@vmware.com
     team_google_chat_group: some-group #! required - google space / chat group for your team
     team_slack_channel: some-channel   #! If your team has slack channel
     ```

3. (optional) Add fields for automatic RMT _**draft**_-release creation

     >**Warning:** You will need to add upgrade specifiers (else Upgrade Planner will break!) and double check your release is ready to be set to GA. It defaults to a draft.

      Release tiles can be used as the basis for automatic `RMT` draft release creation. As a draft this means further steps are required prior to publishing. These include manually setting your upgrade specifiers, double checking the version, GA/EOGs dates, and release type, etc we inferred for you or read from your tile configuration's `rmt` entry.

      See [tile rmt release](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration/tree/main/docs/tile_rmt_release.md) for details.

4. (optional) Add a field to enable automatic Black Duck tile project associations. In order to begin updating your Black Duck tile project:
   >**Prerequisites:** 
   > Follow the [BlackDuck Onboarding section](./creating_open_source_license_disclosures.md#blackduck-onboarding) for
   > your tile. BOSH release projects are managed by GPP for Carvel-based tiles.

   1) Confirm your project exists @ https://broadcom-vmw.app.blackduck.com/ with the format `TNZ-CF-<artifact_name>-tile`, as `<artifact_name>` is found in your tile config.
   2) If not, submit a ticket to request it ([BlackDuck Onboarding section](./creating_open_source_license_disclosures.md#blackduck-onboarding)) or rename it yourself.
   > **Note:** Scanning is enabled by default. However, you may disable it by adding the following to your `./tiles/<tile>.yml` config:
   ```yaml
   blackduck:
        enabled: false
   ```

5. Create a PR to [this](https://github.gwd.broadcom.net/TNZ/tas-ecosystem-configuration) repository to add the newly created files that contain the tile information.  
An extensive PR Check job will verify your change and add a comment if anything needs to be addressed.  When the job passes you can merge the PR.  
On merge, the respective golden path jobs will be created / updated for your tile.  Please reach out to [#tas-slingshots on Google Chat](https://chat.google.com/room/AAAAZuDvKe0?cls=7) with any questions or issues getting your PR merged.

## Updating CI for your tile and automatic RMT _draft_ releases

### Use `kiln carvel publish --final`

If you are using CI to create new versions of tiles, the following updates can be made to take advantage of reproducible builds via `kiln carvel rebake`.

Update your CI to output final tile builds using `kiln carvel publish --final`. When passing the **_--final_** flag, Kiln creates a bake record file under the **_bake_records_** folder. As part of the final tile build CI job, the bake records file needs to be committed and pushed to the tile repository.

Golden Path to publish will then use this bake record to trigger `kiln carvel rebake`, producing a final tile from our [CI](https://runway-ci-srp.eng.vmware.com/teams/tas-ecosystem/) and upload it to [artifactory](https://build-artifactory.eng.vmware.com/ui/repos/tree/General/tas-ecosystem-generic-local/) repo under the sub-path: tile-releases.

In the case of a pre-release version the build will not result in a publish. This is useful to verify OSL triage status and test your candidate build. For example, you may create a
bake record with version `2.4.41-dev.0`, rerun the OSL generation multiple times, then finally create a `2.41.1` to trigger the full publish.

   Example `kiln carvel publish --final` command:

   ```bash
   $ kiln carvel publish --final --version 2.1.41 \
      --output-file my-tile-2.1.41.pivotal \
      --source-directory .
   ```

   _Example:_ Bake record that should be committed to the tile repo that is created by `kiln carvel publish --final` as file: `bake_records/2.1.41.json`:

   ```json
   {
     "source_revision": "1b19d8cb80e6cfdddd7be1c7a26c8210cbd4e4c5",
     "version": "2.1.41",
     "kiln_version": "0.90.0",
     "file_checksum": "7622143c54dc53087a6c2401f5030170515e14f466857564a980092d4c87a094",
     "tile_directory": "."
   }
   ```

When executing `kiln carvel publish --final`, use the values for artifactory variables in your Kilnfile:

- artifactory_host: `https://usw1.packages.broadcom.com`
- artifactory_repo: `tas-ecosystem-generic-prod-local`
- artifactory_username: **_your account or service account for broadcom artifactory_**
- artifactory_password: **_respective api_key or identity token_**

**_NOTE: https://usw1.packages.broadcom.com is accessible via Broadcom VPN with full tunnel gateway and TPE concourse workers_**

**_Commit the new bake record to the git repository of the tile_**

**_Warning: Your bake_records directory must only contain bake records_**

GPP will then automatically run `kiln carvel rebake` against the bake record to produce the final `.pivotal` file, validate the checksum, and upload it to Artifactory and optionally RMT.

Please refer to [Tile RMT Release](Publish-Tiles-to-RMT) to configure publishing your tile to RMT via Golden Path to Publish.

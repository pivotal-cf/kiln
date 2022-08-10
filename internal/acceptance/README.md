# Acceptance test scenarios

## Philosophy

We believe a good acceptance test suite has the following properties:
1. The tests are comphrehendable to new team members
1. The tests are extendable by non-team members
1. The tests describe the intended functioning of the source under test
1. The test coverage is discoverable
1. Test errors facilitate fixing the error
1. The tests may have unit tests

## Structure

We have two sets of acceptance tests using different testing frameworks.

### Bake tests
These are written in Go and use [Gingko+Gomega](https://onsi.github.io/ginkgo/).

```bash
# change directory into the "bake" acceptance test directory then run:
go run github.com/onsi/ginkgo/ginkgo
```

### Workflows
These are written in Go and use [godog](https://github.com/cucumber/godog) (a Cucumber test framework).

```bash
# from anywhere in the repo you can run:
go test -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
```

#### ⚠️ The `generating_release_notes` Requires Access to A Private Repository ⚠️

If you'd like to switch to using a public repository, you can run the following and the test should pass so long as
the public repo is kept up to date with the private repo (no promises here). Please do not commit the following change.

```bash
rm -rf internal/acceptance/workflows/hello-tile
git submodule add --force git@github.com:crhntr/hello-tile internal/acceptance/workflows/hello-tile
```

#### ⚠️ The `caching_compiled_releases` Test Does Not Run in CI ⚠️
This test requires SSH access to an Ops Manager.
Deploying an Ops Manager for an open source repo is not secure.
This one test should be run against a non-production Ops Manager or from a VMware developer's machine on the internal network using a Toolsmiths environment.

<details>
<summary><em>Instructions to test with a Toolsmiths deployed Ops Manager</em></summary>
<br>

Ensure you have the [Smith CLI](https://github.com/pivotal/smith) properly installed and you are logged in.

PPE team members may execute the AWS environment setup expressions in the script.
Non-ppe-team-members may ask us for temporary credentials [generated here](https://console.aws.amazon.com/iam/home#/users/kiln_acceptance_tests?section=security_credentials).
Note the credential created on 2022-08-08 (id ending in "QOV") should not be deleted. It is stored in vault.

```bash
## START Setup
eval "$(smith claim -p us_2_12)"
eval "$(smith bosh)"
eval "$(smith om)"
export OM_PRIVATE_KEY="$(cat $(echo "${BOSH_ALL_PROXY}" | awk -F= '{print $2}'))"

# AWS environment setup
export AWS_ACCESS_KEY_ID="$(vault read --field=aws_access_key_id runway_concourse/ppe-ci/kiln-acceptance-tests-s3)"
export AWS_SECRET_ACCESS_KEY="$(vault read --field=aws_secret_access_key runway_concourse/ppe-ci/kiln-acceptance-tests-s3)"

export GITHUB_TOKEN="$(gh auth status --show-token 2>&1 | grep Token | awk '{print $NF}')"

# optional
export CGO_ENABLED=0
## END Setup

# Run the caching_compiled_releases test
go test --run caching_compiled_releases -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
```
</details>

## Contributing

Please follow existing style and make sure the acceptance unit tests both in the workflows and in the scenario package pass.

# Acceptance test scenarios

## Philosophy

We believe a good acceptance test suite has the following properties:
1. The tests are comprehensible to new team members
1. The tests are extendable by non-team members
1. The tests describe the intended functioning of the source under test
1. The test coverage is discoverable
1. Test errors facilitate fixing the error
1. The tests may have unit tests

## Structure

We have three sets of acceptance tests. All of them are behind the `acceptance` build tag,
so `go test ./...` does not compile them and they never run in the unit suite.

### Bake tests
These are written in Go and use [Gingko+Gomega](https://onsi.github.io/ginkgo/).

```bash
# change directory into the "bake" acceptance test directory then run:
go run github.com/onsi/ginkgo/v2/ginkgo --tags acceptance
```

### Workflows
These are written in Go and use [godog](https://github.com/cucumber/godog) (a Cucumber test framework).

> PS: Export GITHUB_ACCESS_TOKEN as an env var before running the acceptance tests

```bash
# from anywhere in the repo you can run:
export GITHUB_ACCESS_TOKEN="$(gh auth token)"
go test -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
```

### Carvel
These drive `kiln carvel bake/upload/publish/rebake` end to end against a mock Artifactory,
and require the [bosh CLI](https://bosh.io/docs/cli-v2-install/) on your PATH.

```bash
# from anywhere in the repo you can run:
go test -v --tags acceptance --timeout=15m github.com/pivotal-cf/kiln/internal/acceptance/carvel
```

## Running everything the way CI does

```bash
export GITHUB_ACCESS_TOKEN="$(gh auth token)"
go test -v --timeout 15m --tags acceptance \
  github.com/pivotal-cf/kiln/internal/acceptance/workflows \
  github.com/pivotal-cf/kiln/internal/acceptance/carvel \
  github.com/pivotal-cf/kiln/internal/acceptance/bake
```

## Contributing

Please follow existing style and make sure the acceptance unit tests both in the workflows and in the scenario package pass.

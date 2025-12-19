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

We have two sets of acceptance tests using different testing frameworks.

### Bake tests
These are written in Go and use [Gingko+Gomega](https://onsi.github.io/ginkgo/).

```bash
# change directory into the "bake" acceptance test directory then run:
go run github.com/onsi/ginkgo/v2/ginkgo
```

### Workflows
These are written in Go and use [godog](https://github.com/cucumber/godog) (a Cucumber test framework).

> PS: Export GITHUB_ACCESS_TOKEN as an env var before running the acceptance tests

```bash
# from anywhere in the repo you can run:
export GITHUB_ACCESS_TOKEN="$(gh auth token)"
go test -v --tags acceptance --timeout=1h github.com/pivotal-cf/kiln/internal/acceptance/workflows
```

## Contributing

Please follow existing style and make sure the acceptance unit tests both in the workflows and in the scenario package pass.

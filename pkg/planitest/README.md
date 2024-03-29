# planitest

## What is it?

Test helpers for Ops Manager tile developers. Given the set of tile options selected by the operator, what should the generated BOSH manifest look like?

It can be prohibitively expensive to deploy your tile in each of these configurations - planitest lets you make assertions about the staged manifest.

## Usage

See the [tested example](example_product_service_test.go)

## What do you need?

There are two ways to run planitest, using a real Ops Manager as backend
renderer or using a generator tool to provide faster feedback.

### Use `om` as renderer
1. Set environment variable `RENDERER` to `om`
1. An [Ops Manager](https://docs.vmware.com/en/VMware-Tanzu-Operations-Manager/2.10/vmware-tanzu-ops-manager/index.html) instance to test against. It should have the BOSH tile deployed.
1. The [om](https://github.com/pivotal-cf/om) CLI, n.b. requires `om` 0.42.0+
1. The [bosh](https://bosh.io/docs/cli-v2.html#install) CLI
1. A config file usable by `om configure-product`, see `om` [documentation](https://github.com/pivotal-cf/om/tree/main/docs/configure-product#example-yaml)
1. The tile you want to test. It should be already uploaded to Ops Manager, along with the stemcell it depends on.

#### Rough Edges for `om`:
1. Don't attempt to run tests in parallel as different examples will step on each other
1. It runs om with the `--skip-ssl-validation` flag
1. Rendering a staged manifest for a large product on Ops Manager can be **slooooow**

### Use `ops-manifest` as renderer
1. Set environment variable `RENDERER` to `ops-manifest`
1. The [ops-manifest](https://github.com/pivotal-cf/ops-manifest) CLI
1. The metadata.yml file extracted from a tile
1. A configuration file exported with [`om staged-config`](https://github.com/pivotal-cf/om/blob/main/docs/staged-config/README.md)

#### Rough edges for `ops-manifest`:
1. `ops-manifest` is also under heavy construction; it may render differently from an Ops Manager
1. Config file may be hard to configure. It requires appropriate `product-properties` and `network-properties` fields.
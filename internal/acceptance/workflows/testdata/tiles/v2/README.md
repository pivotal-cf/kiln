# A [Ops Manager Tile](https://docs.pivotal.io/tiledev/2-10/index.html) wrapping [Hello Release](https://github.com/releen/hello-release) [![Release](https://github.com/releen/hello-tile/actions/workflows/release.yml/badge.svg)](https://github.com/releen/hello-tile/actions/workflows/release.yml)

This is an example for [Kiln](https://github.com/pivotal-cf/kiln).

It has an automated release pipeline to create ".pivotal" files and adds them to GitHub releases.


## Testing

### Manifest Tests

You can run kiln test; however, Kiln test runs in a container so you need to vendor Go dependencies before you run it.

To run tests,
- clone the repository
- fetch a version of kiln that supports test
- run `go vendor`
- run `kiln test`

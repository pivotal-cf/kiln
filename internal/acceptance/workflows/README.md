# Acceptance test scenarios

## TODO: describe our testing philosopy is like CATs
## TODO: describe our technology stack for accomplshing this
## TODO: add a contributor's guide

## List of first-class workflows we care about
1. Human
  - Update functionality in a Tile

2. Robot
  - Publish the Tile
    - Fetch uncompiled releases
    - Fetch compiled releases
    - Run Tile unit tests
    - Build the Tile
    - Outside-of-Kiln work here
       - Create Pivnet Release
       - Upload the Tile to Pivnet
    - Publish Pivnet Release
       - `kiln publish` updates Pivnet Release metadata to make it visible
    - Create Release Notes
      - Not Kiln: `git clone` release notes
      - `kiln release-notes`
      - Not Kiln: `git commit` release notes
      - Not Kiln: `git push` release notes

3. Both
  - Build the Tile

  - Update a release version in the Tile
  - Update the stemcell for a Tile

  - Update release notes for a Tile

# Acceptance Testing Philosophy

We believe a good acceptance test suite has the following properties:
1. The tests are comphrehendable to new team members
1. The tests are extendable by non-team members
1. The tests describe the intended functioning of the source under test
1. The test coverage is discoverable
1. Test errors facilitate fixing the error

## Open Philosophy Questions
1. Do we unit test the acceptance tests?

## Can we test Tiles Cucumber-style?
`kiln test <directory>`
`godog run <directory>`



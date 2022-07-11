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

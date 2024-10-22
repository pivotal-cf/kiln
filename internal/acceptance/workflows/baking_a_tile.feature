Feature: As a developer, I want to bake a tile
  Scenario: it fetches components and bakes a tile
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    When I invoke kiln
      | bake                                      |
      | --final                                   |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.2.12.tgz           |
      | releases/hello-release-0.2.3.tgz |
    And "bake_records/0.2.0-dev.json" contains substring: "version": "0.2.0-dev"
    And "bake_records/0.2.0-dev.json" contains substring: "source_revision": "896c44a006a24d8601ed09fd871b1a4423636d77"
    And "bake_records/0.2.0-dev.json" contains substring: "tile_directory": "."
    And "bake_records/0.2.0-dev.json" contains substring: "kiln_version": "0.0.0+acceptance-tests"
    And "bake_records/0.2.0-dev.json" contains substring: "file_checksum": "6754bb95193e42cd5706f810c3fdb1beead88e2a01601bb222e3e98818f90e8a"
    And "tile-0.2.0-dev.pivotal" has sha256 sum "6754bb95193e42cd5706f810c3fdb1beead88e2a01601bb222e3e98818f90e8a"

  Scenario: it reads directory configuration from Kilnfile
    Given I have a tile source directory "testdata/tiles/non-standard-paths"
    When I invoke kiln
      | bake            |
      | --stub-releases |
    Then a Tile is created

  Scenario: it handles tiles with multiple tile names
    Given I have a tile source directory "testdata/tiles/multiple-tile-names"
    When I invoke kiln
      | bake                |
      | --tile-name=goodbye |
    Then a Tile is created

  Scenario: it bakes a tile from a bake record
    Given I have a tile source directory "testdata/tiles/bake-record"
    When I invoke kiln
      | re-bake                          |
      | --output-file=tile-0.1.0.pivotal |
      | tile/bake_records/0.1.0.json          |
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml   |
      | releases/bpm-1.1.21.tgz |
Feature: As a developer, I want to bake a tile
  Scenario: it fetches components and bakes a tile
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    When I invoke kiln
      | bake                                      |
      | --final                                   |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.2.12.tgz           |
      | releases/hello-release-0.2.3.tgz |
    And "bake_records/0.2.0-dev.json" contains substring: "version": "0.2.0-dev"
    And "bake_records/0.2.0-dev.json" contains substring: "source_revision": "bc3ac24e192ba06a2eca19381ad785ec7069e0d0"
    And "bake_records/0.2.0-dev.json" contains substring: "tile_directory": "."
    And "bake_records/0.2.0-dev.json" contains substring: "kiln_version": "0.0.0+acceptance-tests"
    And "bake_records/0.2.0-dev.json" contains substring: "file_checksum": "98239fa2fad3132c4cc12407f0f6c77bdcae1faec00fb4ef4c2b420637522db4"
    And "tile-0.2.0-dev.pivotal" has sha256 sum "98239fa2fad3132c4cc12407f0f6c77bdcae1faec00fb4ef4c2b420637522db4"

  Scenario: it reads directory configuration from Kilnfile
    Given I have a tile source directory "testdata/tiles/non-standard-paths"
    When I invoke kiln
      | bake            |
      | --stub-releases |
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
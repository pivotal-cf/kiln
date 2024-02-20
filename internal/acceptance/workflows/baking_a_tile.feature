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
    And "bake_records/0.2.0-dev.json" contains substring: "kiln_version": "0.0.0+acceptance-tests"
    And "tile-0.2.0-dev.pivotal" has sha256 sum "ec0f718ffcf2f066024b4bb6ae0249677e641c4753e28b0ca6d8b8ea6b2e29c5"

  Scenario: it reads directory configuration from Kilnfile
    Given I have a tile source directory "testdata/tiles/non-standard-paths"
    When I invoke kiln
      | bake            |
      | --stub-releases |
    Then a Tile is created

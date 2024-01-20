Feature: As a developer, I want to bake a tile
  Scenario: it fetches components and bakes a tile
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    When I invoke kiln
      | bake                                      |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.2.12.tgz           |
      | releases/hello-release-0.2.3.tgz |
    And "bake_records/0.2.0-dev.json" contains substring: "version": "0.2.0-dev"
    And "bake_records/0.2.0-dev.json" contains substring: "source_revision":
    And "bake_records/0.2.0-dev.json" contains substring: "kiln_version": "0.0.0+acceptance-tests"

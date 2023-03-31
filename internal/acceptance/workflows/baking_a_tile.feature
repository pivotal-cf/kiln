Feature: As a developer, I want to bake a tile
  Scenario: it fetches components and bakes a tile
    Given I have a "hello-tile" repository checked out at v0.1.7
    And the repository has no fetched releases
    When I invoke kiln
      | bake                                      |
      | --version=0.1.7                           |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.1.18.tgz           |
      | releases/hello-release-0.1.5.tgz |

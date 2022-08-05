Feature: Baking
  Scenario: it fetches components and bakes a tile
    Given I have a "hello-tile" repository checked out at v0.1.1
    And the repository has no fetched releases
    When I invoke kiln fetch
    And I invoke kiln bake
    Then a Tile is created
    And the Tile contains
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.1.18.tgz           |
      | releases/hello-release-v0.1.3.tgz |

Feature: Bake a Tile
  Scenario: `kiln bake` bakes a Tile
    Given I have a "hello-tile" repository checked out at v0.1.1
    When I fetch releases
    And I invoke kiln bake
    Then a Tile is created
    And the Tile contains "<filepath>"
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.1.18.tgz           |
      | releases/hello-release-v0.1.3.tgz |

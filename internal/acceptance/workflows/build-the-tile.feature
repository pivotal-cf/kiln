Feature: Bake a Tile
  Scenario Outline: `kiln bake` bakes a Tile
    Given I have a "hello-tile" repository checked out at v0.1.0
    When I invoke `kiln bake`
    Then a Tile is created
    And the Tile contains "<filepath>"
      | filepath                          |
      | metadata/metadata.yml             |
      | migrations/v1                     |
      | releases/bpm-1.1.18.tgz           |
      | releases/hello-release-v0.1.0.tgz |

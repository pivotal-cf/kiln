Feature: Cache Compiled Releases
  Scenario: kiln cache-compiled-release stores compiled releases in an S3 bucket
    Given I have a "hello-tile" repository checked out at v0.1.2
    And I invoke kiln fetch
    And I invoke kiln bake
    And I upload, configure, and apply the tile
    And I add a compiled s3 release-source "hello-tile-releases" to the Kilnfile
    And the stemcell version in the lock matches the used for the tile
    When I invoke kiln cache-compiled-releases
    And the repository has no fetched releases
    And I invoke kiln fetch
    And I invoke kiln bake
    Then a Tile is created
    And the Tile only contains compiled releases

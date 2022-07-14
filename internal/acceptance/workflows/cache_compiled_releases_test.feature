Feature: Cache Compiled Releases
  Scenario: kiln cache-compiled-release stores compiled releases in an S3 bucket
    Given I have a smith environment
    And I have a "hello-tile" repository checked out at v0.1.2
    And the repository has no fetched releases
    And I invoke kiln fetch
    And I invoke kiln bake
    And I upload, configure, and apply the tile with stemcell ubuntu-xenial/621.256
    And I add a compiled s3 release-source "hello-tile-releases" to the Kilnfile
    When I invoke kiln cache-compiled-releases
    And I fetch releases
    And I invoke kiln bake
    Then a Tile is created
    And the Tile contains compiled releases

Feature: As a maintainer, I want to pin all BOSH Releases
  Scenario: Kilnfile releases are floating
    Given I have a tile source directory "testdata/tiles/v2"
    When I invoke kiln
      | glaze |
    Then the Kilnfile version for release "hello-release" is "0.4.0"
    And  the Kilnfile version for release "bpm" is "1.2.12"
    And  the Kilnfile version for the stemcell is "1.329"



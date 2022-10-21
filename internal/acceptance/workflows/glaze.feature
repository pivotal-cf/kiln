Feature: As a maintainer, I want to pin all BOSH Releases
  Scenario: Kilnfile releases are floating
    Given I have a "hello-tile" repository checked out at v0.1.1
    When I invoke kiln
      | glaze |
    Then the Kilnfile version for release "hello-release" is "v0.1.3"
    And  the Kilnfile version for release "bpm" is "1.1.18"
    And  the Kilnfile version for the stemcell is "621.0"
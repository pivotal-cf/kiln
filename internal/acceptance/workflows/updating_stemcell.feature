Feature: As a dependabot, I want to update a stemcell

  # This test is brittle. When a new stemcell is released, this will fail.
  # We need to fix the stemcell logic to respect the stemcell version constraint.
  # Until we do, we need to update the expectations in this file.

  Scenario: Find the new stemcell
    Given I have a tile source directory "testdata/tiles/v2"
    And TanzuNetwork has product "stemcells-ubuntu-jammy" with version "1.340"
    And I set the Kilnfile stemcell version constraint to "=< 1.341"
    When I invoke kiln
      | find-stemcell-version                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout contains substring: "1.340"

  Scenario: Update the stemcell
    Given I have a tile source directory "testdata/tiles/v2"
    And TanzuNetwork has product "stemcells-ubuntu-jammy" with version "1.340"
    And "Kilnfile.lock" contains substring: version: "1.329"
    When I invoke kiln
      | update-stemcell                           |
      | --version=1.340                         |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then "Kilnfile.lock" contains substring: version: "1.340"

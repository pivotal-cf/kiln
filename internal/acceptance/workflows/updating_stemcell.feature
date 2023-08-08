Feature: As a dependabot, I want to update a stemcell

  # This test is brittle. When a new stemcell is released, this will fail.
  # We need to fix the stemcell logic to respect the stemcell version constraint.
  # Until we do, we need to update the expectations in this file.

  Scenario: Find the new stemcell
    Given I have a valid "hello-tile" repository
    And TanzuNetwork has product "stemcells-ubuntu-xenial" with version "621.418"
    And I set the Kilnfile stemcell version constraint to "<621.419"
    When I invoke kiln
      | find-stemcell-version                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout contains substring: "621.418"

  Scenario: Update the stemcell
    Given I have a valid "hello-tile" repository
    And TanzuNetwork has product "stemcells-ubuntu-xenial" with version "621.330"
    When I invoke kiln
      | update-stemcell                           |
      | --version=621.330                         |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then "./hello-tile/Kilnfile.lock" contains substring: version: "621.330"

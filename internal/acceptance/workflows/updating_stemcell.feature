Feature: As a dependabot, I want to update a stemcell

  # This test is brittle. When a new stemcell is released, this will fail.
  # We need to fix the stemcell logic to respect the stemcell version constraint.
  # Until we do, we need to update the expectations in this file.

  Scenario: Find the new stemcell
    Given I have a "hello-tile" repository checked out at v0.1.5
    And the environment variable "GITHUB_TOKEN" is set
    When I invoke kiln
      | find-stemcell-version                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout is valid json

  Scenario: Update the stemcell
    Given I have a "hello-tile" repository checked out at v0.1.5
    And the environment variable "GITHUB_TOKEN" is set
    And TanzuNetwork has product "stemcells-ubuntu-xenial" with version "621.280"
    And the Kilnfile.lock specifies version "621.0" for the stemcell
    When I invoke kiln
      | update-stemcell                           |
      | --version=621.261                         |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then "./hello-tile/Kilnfile.lock" contains substring: version: "621.261"

Feature: As a dependabot, I want to update a stemcell

  Scenario: Find the new stemcell
    Given I have a "hello-tile" repository checked out at v0.1.5
    And TanzuNetwork has product "stemcells-ubuntu-xenial" with version "621.261"
    And I set the Kilnfile stemcell version constraint to "<=621.261"
    When I invoke kiln
      | find-stemcell-version                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout contains substring: "621.261"

  Scenario: Update the stemcell
    Given I have a "hello-tile" repository checked out at v0.1.5
    And TanzuNetwork has product "stemcells-ubuntu-xenial" with version "621.261"
    And I set the Kilnfile stemcell version constraint to "<=621.261"
    And the Kilnfile.lock specifies version "621.0" for the stemcell
    When I invoke kiln
      | update-stemcell                           |
      | --version=621.261                         |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then "./hello-tile/Kilnfile.lock" contains substring: version: "621.261"
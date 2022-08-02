Feature: Update a stemcell

  Scenario: Find the new stemcell
    Given I have a "hello-tile" repository checked out at v0.1.1
    And the Kilnfile.lock specifies version 621.0 for the stemcell
    And pivnet has a newer stemcell
    When I invoke kiln find-stemcell-version
    Then I find a stemcell with a higher version

  Scenario: Update the stemcell
    Given I have a "hello-tile" repository checked out at v0.1.1
    And the Kilnfile.lock specifies version 621.0 for the stemcell
    And pivnet has a newer stemcell
    When I invoke kiln update-stemcell with the newer version
    Then the Kilnfile.lock specifies the newer stemcell version
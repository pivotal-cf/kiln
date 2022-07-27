Feature: Update a Tile Component
  Scenario: Find new releases
    Given I have a "hello-tile" repository checked out at v0.1.1
    And the Kilnfile.lock specifies version 0.1.3 for release hello-release
    And hello-release has a version 0.1.4
    When I invoke kiln find-release-version for hello-release
    Then I find hello-release with version 0.1.4
    And kiln validate succeeds
  Scenario: Update a component to a new release
    Given I have a "hello-tile" repository checked out at v0.1.1
    And the Kilnfile.lock specifies version 0.1.3 for release hello-release
    And hello-release has a version 0.1.4
    When I invoke kiln update-release for hello-release with version 0.1.4
    Then the Kilnfile.lock specifies version 0.1.4 for release hello-release
    And kiln validate succeeds
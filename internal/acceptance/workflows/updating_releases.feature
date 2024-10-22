Feature: As a dependabot, I want to update a BOSH Release
  Scenario: Find a version on GitHub
    Given I have a tile source directory "testdata/tiles/v2"
    And GitHub repository "crhntr/hello-release" has release with tag "v0.2.3"
    And I set the version constraint to "0.2.3" for release "hello-release"
    When I invoke kiln
      | find-release-version                      |
      | --release=hello-release                   |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then stdout contains substring: "0.2.3"

  Scenario: Find a version on bosh.io
    Given I have a tile source directory "testdata/tiles/v2"
    And I set the version constraint to "1.1.18" for release "bpm"
    When I invoke kiln
      | find-release-version                      |
      | --release=bpm                             |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then stdout contains substring: "1.1.18"

  Scenario: Update a component to a new release
    Given I have a tile source directory "testdata/tiles/v2"
    And the Kilnfile.lock specifies version "0.2.3" for release "hello-release"
    And GitHub repository "crhntr/hello-release" has release with tag "v0.2.3"
    When I invoke kiln
      | update-release                            |
      | --name=hello-release                      |
      | --version=v0.2.3                          |
      | --without-download                        |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then the Kilnfile.lock specifies version "0.2.3" for release "hello-release"
    And kiln validate succeeds

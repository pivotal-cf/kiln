Feature: As a dependabot, I want to update a BOSH Release
  Scenario: Find a version on GitHub
    Given I have a valid "hello-tile" repository
    And GitHub repository "crhntr/hello-release" has release with tag "v0.1.4"
    When I invoke kiln
      | find-release-version                      |
      | --release=hello-release                   |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout contains substring: "0.2.3"

  Scenario: Find a version on bosh.io
    Given I set the version constraint to "1.1.18" for release "bpm"
    When I invoke kiln
      | find-release-version                      |
      | --release=bpm                             |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then stdout contains substring: "1.1.18"

  Scenario: Update a component to a new release
    Given I have a valid "hello-tile" repository
    And the Kilnfile.lock specifies version "v0.1.4" for release "hello-release"
    And GitHub repository "crhntr/hello-release" has release with tag "v0.1.5"
    When I invoke kiln
      | update-release                            |
      | --name=hello-release                      |
      | --version=0.1.5                           |
      | --without-download                        |
      | --variable=github_token="${GITHUB_TOKEN}" |
    Then the Kilnfile.lock specifies version "0.1.5" for release "hello-release"
    And kiln validate succeeds

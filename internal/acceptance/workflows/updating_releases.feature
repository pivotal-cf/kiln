Feature: As a dependabot, I want to update a BOSH Release
  Scenario: Find a version on GitHub
    Given I have a tile source directory "testdata/tiles/v2"
    And GitHub repository "releen/hello-release" has release with tag "0.4.0"
    And I set the version constraint to "0.4.0" for release "hello-release"
    When I invoke kiln
      | find-release-version                      |
      | --release=hello-release                   |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then stdout contains substring: "0.4.0"

  Scenario: Find a version on bosh.io
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    And I set the version constraint to "1.1.18" for release "bpm"
    When I invoke kiln
      | find-release-version                      |
      | --release=bpm                             |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then stdout contains substring: "1.1.18"

  Scenario: Update a component to a new release with download
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    And the Kilnfile.lock specifies version "0.4.0" for release "hello-release"
    And GitHub repository "releen/hello-release" has release with tag "0.5.0"
    When I invoke kiln
      | update-release                            |
      | --name=hello-release                      |
      | --version=0.5.0                           |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then the Kilnfile.lock specifies version "0.5.0" for release "hello-release"
    And kiln validate succeeds
    And the "hello-release-0.5.0.tgz" release tarball exists

  Scenario: Update a component to a new release without download
    Given I have a tile source directory "testdata/tiles/v2"
    And the repository has no fetched releases
    And the Kilnfile.lock specifies version "1.2.12" for release "bpm"
    When I invoke kiln
      | update-release                            |
      | --name=bpm                                |
      | --version=1.2.13                          |
      | --without-download                        |
      | --variable=github_access_token="${GITHUB_ACCESS_TOKEN}" |
    Then the Kilnfile.lock specifies version "1.2.13" for release "bpm"
    And kiln validate succeeds
    And the "bpm-1.2.13.tgz" release tarball does not exist

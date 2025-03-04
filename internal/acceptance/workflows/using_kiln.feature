Feature: As a developer, I want the Kiln CLI to be usable
  Scenario: I ask for the version
    Given I have a tile source directory "testdata/tiles/v2"
    When I invoke kiln
      | version |
    Then the exit code is 0
    And stdout contains substring: 0.0.0+acceptance-tests
  Scenario: I ask for help
    Given I have a tile source directory "testdata/tiles/v2"
    When I invoke kiln
      | help |
    Then the exit code is 0
    And stdout contains substring: kiln helps you
    And stdout contains substring: Usage:
    And stdout contains substring: Commands:

  Scenario: I mess up my command name
    Given I have a tile source directory "testdata/tiles/v2"
    When I try to invoke kiln
      | boo-boo |
    # TODO: in this case we should expect output on stderr not stdout
    Then the exit code is 1
    And stderr contains substring: unknown command

  Scenario Outline: I mess up my command flags
    Given I have a tile source directory "testdata/tiles/v2"
    When I try to invoke kiln
      | <command> |
      | --boo-boo |
    Then the exit code is 1
    And stderr contains substring: flag provided but not defined

    Examples:
      | command                 |
      | bake                    |
      | re-bake                 |
      | fetch                   |
      | find-release-version    |
      | find-stemcell-version   |
      | publish                 |
      | release-notes           |
      | sync-with-local         |
      | update-release          |
      | update-stemcell         |
      | validate                |

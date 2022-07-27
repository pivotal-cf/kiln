Feature: Kiln CLI is self documenting
  Scenario: I ask for the version
    Given I care about stdout
    When I invoke kiln version
    Then I get version output on stdout
  Scenario: I ask for help
    Given I care about stdout
    When I invoke kiln help
    Then I get help output on stdout
  Scenario: I mess up my command name
    # TODO: in this case we should care about stderr not stdout
    Given I care about stdout
    When I invoke kiln boo-boo
    Then I get help output on stdout
  Scenario: I fuck up my command flags
    # TODO: in this case we should care about stderr not stdout
    Given I care about stdout
    When I invoke kiln <command> --boo-boo
      | bake                    |
      | cache-compiled-releases |
      | fetch                   |
      | find-release-version    |
      | find-stemcell-version   |
      | help                    |
      | publish                 |
      | release-notes           |
      | sync-with-local         |
      | update-release          |
      | update-stemcell         |
      | upload-release          |
      | validate                |
      | version                 |
    Then I get help output on stdout
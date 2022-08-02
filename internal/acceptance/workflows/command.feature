Feature: Kiln CLI is self documenting
  Scenario: I ask for the version
    When I invoke kiln version
    Then stdout contains substring: 1.0.0-dev
  Scenario: I ask for help
    When I invoke kiln help
    Then I get help output on stdout
  Scenario: I mess up my command name
    When I invoke kiln boo-boo
    # TODO: in this case we should expect output on stderr not stdout
    Then I get help output on stdout
  Scenario: I mess up my command flags
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
    # TODO: in this case we should expect output on stderr not stdout
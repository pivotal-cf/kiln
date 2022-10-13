Feature: As a robot, I want to generate release notes
  Scenario: Update exising release notes document
    Given I have a "hello-tile" repository checked out at v0.1.4
    And GitHub repository "crhntr/hello-release" has release with tag "v0.1.5"
    And the environment variable "GITHUB_TOKEN" is set

    # it does not contain 0.1.4 release header only 0.1.3
    And "./scenario/fixtures/release_notes.md.erb" has regex matches: id='(?P<version>[\d\.]+)'
      | version |
      | 0.1.3   |

    When I invoke kiln
      | create-release-notes                                    |
      | --release-date=2022-07-27                               |
      | --github-issue-milestone=Release-2022-001               |
      | --update-docs=../scenario/fixtures/release_notes.md.erb |
      | --kilnfile=Kilnfile                                     |
      | v0.1.3                                                  |
      | v0.1.4                                                  |

    # it contains the release header IDs
    Then "./scenario/fixtures/release_notes.md.erb" has regex matches: id='(?P<version>[\d\.]+)'
      | version |
      | 0.1.4   |
      | 0.1.3   |

    # it contains the issue title
    And "./scenario/fixtures/release_notes.md.erb" contains substring: **[Bug Fix]** Index page has inconsistent whitespace

    # it contains the component release note
    And "./scenario/fixtures/release_notes.md.erb" contains substring: "**[Fix]**\n  The HTML had  inconsistent	 spacing"

    # the table contains the name/versions
    And "./scenario/fixtures/release_notes.md.erb" has regex matches: (?mU)<tr><td>(?P<release_name>.*)</td><td>(?P<release_version>.*)</td>
      | release_name           | release_version |
      | ubuntu-xenial stemcell | 621.0           |
      | bpm                    | 1.1.18          |
      | hello-release          | 0.1.5           |
      | ubuntu-xenial stemcell | 621.0           |
      | bpm                    | 1.1.18          |
      | hello-release          | v0.1.4          |
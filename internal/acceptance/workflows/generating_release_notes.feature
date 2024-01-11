Feature: As a robot, I want to generate release notes
  Scenario: Update exising release notes document
    Given I have a tile source directory "testdata/tiles/v1"

    And I execute git tag v0.1.3

    And GitHub repository "crhntr/hello-release" has release with tag "v0.1.5"
    And I invoke kiln
      | update-release                            |
      | --name=hello-release                      |
      | --version=v0.1.5                          |
      | --without-download                        |
      | --variable=github_token="${GITHUB_TOKEN}" |
    And I write file "version"
      | 0.1.4 |
    And I execute git add Kilnfile.lock version
    And I execute git commit -m bump-hello
    And I execute git show
    And I execute git tag v0.1.4

    And I execute git remote add origin git@github.com:crhntr/hello-tile

    # it does not contain 0.1.4 release header only 0.1.3
    And "notes/release_notes.md.erb" has regex matches: id='(?P<version>[\d\.]+)'
      | version |
      | 0.1.3   |

    And the environment variable "GITHUB_TOKEN" is set

    When I invoke kiln
      | release-notes                             |
      | --release-date=2022-07-27                 |
      | --github-issue-milestone=Release-2022-001 |
      | --update-docs=notes/release_notes.md.erb  |
      | --kilnfile=Kilnfile                       |
      | v0.1.3                                    |
      | v0.1.4                                    |

    # it contains the release header IDs
    Then "notes/release_notes.md.erb" has regex matches: id='(?P<version>[\d\.]+)'
      | version |
      | 0.1.4   |
      | 0.1.3   |

    # it contains the issue title
    And "notes/release_notes.md.erb" contains substring: **[Bug Fix]** Index page has inconsistent whitespace

    # it contains the component release note
    And "notes/release_notes.md.erb" contains substring: "**[Fix]**\n  The HTML had  inconsistent	 spacing"

    # the table contains the name/versions
    And "notes/release_notes.md.erb" has regex matches: (?mU)<tr><td>(?P<release_name>.*)</td><td>(?P<release_version>.*)</td>
      | release_name           | release_version |
      | ubuntu-xenial stemcell | 621.0           |
      | bpm                    | 1.1.18          |
      | hello-release          | 0.1.5           |
      | ubuntu-xenial stemcell | 621.0           |
      | bpm                    | 1.1.18          |
      | hello-release          | v0.1.4          |
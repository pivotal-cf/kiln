Feature: Generating Release Notes
  Scenario: A milestone is provided
    Given I have a "hello-tile" repository checked out at v0.1.4
    And GitHub repository "crhntr/hello-release" has release with tag "v0.1.5"

    When I invoke kiln release-notes "v0.1.3" "v0.1.4"

    Then "./release_notes.md.erb" contains substring: ### <a id='0.1.4'></a> 0.1.4
    And "./release_notes.md.erb" contains substring: **Release Date:** 07/27/2022
    And "./release_notes.md.erb" contains substring: * **[Bug Fix]** Index page has inconsistent whitespace
    And "./release_notes.md.erb" contains substring: The HTML had  inconsistent	 spacing
    And "./release_notes.md.erb" contains substring: ### <a id='0.1.3'></a> 0.1.3
    # TODO: this does not look right. I think it should add the stemcell used by the compiled releases... maybe.
    And "./release_notes.md.erb" contains substring: <tr><td>ubuntu-xenial stemcell</td><td>621.0</td><td></td></tr>
    And "./release_notes.md.erb" contains substring: <tr><td>bpm</td><td>1.1.18</td><td></td></tr>
    And "./release_notes.md.erb" contains substring: <tr><td>hello-release</td><td>0.1.5</td>
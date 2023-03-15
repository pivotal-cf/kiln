Feature: As a robot, I want to run manifest tests
  Scenario: manifest tests are successful
    Given I have a manifest
    When I run manifest tests
    Then the tests should pass

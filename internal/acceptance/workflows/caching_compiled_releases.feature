Feature: As a robot, I want to cache compiled releases
  Scenario: it stores compiled releases in an S3 bucket
    Given I have a "hello-tile" repository checked out at v0.1.2
    And the environment variable "GITHUB_TOKEN" is set
    And the environment variable "OM_USERNAME" is set
    And the environment variable "OM_PASSWORD" is set
    And the environment variable "OM_TARGET" is set
    And the environment variable "OM_PRIVATE_KEY" is set
    And the environment variable "BOSH_ALL_PROXY" is set
    And the environment variable "AWS_ACCESS_KEY_ID" is set
    And the environment variable "AWS_SECRET_ACCESS_KEY" is set
    And I remove all the objects in the bucket "hello-tile-releases"
    And I invoke kiln
      | fetch                                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    And I invoke kiln
      | bake            |
      | --version=0.1.2 |
    And I upload, configure, and apply the tile
    And I add a compiled s3 release-source "hello-tile-releases" to the Kilnfile
    And I set the stemcell version in the lock to match the one used for the tile
    When I invoke kiln
      | cache-compiled-releases                   |
      | --upload-target-id=hello-tile-releases    |
      | --name=hello                              |
      | --variable=github_token="${GITHUB_TOKEN}" |
    And the repository has no fetched releases
    And I invoke kiln
      | fetch                                     |
      | --variable=github_token="${GITHUB_TOKEN}" |
    And I invoke kiln
      | bake            |
      | --version=0.1.2 |
    Then a Tile is created
    And the Tile only contains compiled releases

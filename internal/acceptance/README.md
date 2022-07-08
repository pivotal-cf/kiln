# Acceptance Tests

## Current Tests

### init_test.go [not tests]
This does suite setup.

### bake_test.go

Considers log output as part of the API but otherwise is cromulent.

### fixtures_test.go [not tests]

not a test.

### help_test.go

These tests over-promise and under deliver.
They only test two commands and badly.

### match_sha_sum_of_matcher_test.go [not tests]

this is a gomega matcher implementation

### stub.go [not tests]

effectively empty

### sync_with_local_test.go

This command is no longer used by our CI.

### version_test.go

This tests all three ways you can get the kiln version.

# Acceptance Tests
There is not much about the structure of these tests to document.  They're
vanilla Gomega test suites, with a normal amount of quirk (like files named
"test" that don't test anything).

## Current Tests
### bake_test.go
Considers log output as part of the API but otherwise is cromulent.

### help_test.go
Named `help_test`, but only tests two commands and badly (i.e. exact string
matches).

### sync_with_local_test.go
This command is no longer used by our CI. We plan to remove it without warning.

### version_test.go
This tests all three ways you can get the kiln version.

## Not Tests
### init_test.go
This does suite setup.

### fixtures_test.go
Declares global variables only used in `bake_test`

### match_sha_sum_of_matcher_test.go
Gomega matcher implementation

### stub.go
Empty Go file

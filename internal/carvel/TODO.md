# Carvel Package TODOs

## Refactor: Replace exec.Command("kiln bake") with internal function call

**File:** `baker.go` (line 46)

**Current behavior:** `KilnBake()` shells out to the system `kiln` binary via
`exec.Command("kiln", "bake", "--skip-fetch", "--output-file", destination)`.

**Problem:**
- The inner `kiln bake` resolves to whatever binary is on PATH, which may be a
  different version than the running `kiln carvel bake`.
- Integration tests must manipulate PATH to ensure the correct binary is used.
- Spawning a subprocess for logic that exists in the same codebase is unnecessary
  overhead.
- Error propagation across the process boundary is lossy.

**Desired behavior:** `KilnBake()` should call the internal bake logic directly
(e.g. instantiate and invoke `commands.Bake` or the underlying `BakeService`)
instead of shelling out. This guarantees version consistency, improves
testability, and removes the PATH dependency.

**Complexity note:** The `Bake` command has a non-trivial setup (BakeService,
fetchers, template evaluators, checksummers). The wiring will need to be
extracted into a reusable helper or the relevant subset of bake logic factored
out for in-process use.

// Package acceptance holds the bake acceptance suite. The specs live behind the
// acceptance build tag because they drive the kiln binary; this file keeps the
// package visible to `go list`/`go build` without that tag.
package acceptance

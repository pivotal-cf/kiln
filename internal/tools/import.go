//go:build tools
// +build tools

// Package tools is a convention in Go codebases to trick the dependency manager to include development CLIs into
// the codebase. It is often used for generators.
package tools

import (
	_ "github.com/maxbrunsfeld/counterfeiter/v6"
)

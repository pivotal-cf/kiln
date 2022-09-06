package component

import (
	"io"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Spec = cargo.ReleaseSpec

type Local struct {
	Lock
	LocalPath string
}

type Lock = cargo.ReleaseLock

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

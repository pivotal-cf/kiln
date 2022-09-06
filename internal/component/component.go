package component

import (
	"io"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Spec = cargo.ComponentSpec

type Local struct {
	Lock
	LocalPath string
}

type Lock = cargo.ComponentLock

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

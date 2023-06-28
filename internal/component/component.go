package component

import (
	"io"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Exported struct {
	Lock        cargo.ComponentLock
	TarballPath string
	BlobstoreID string
}

type Local struct {
	Lock      cargo.ComponentLock
	LocalPath string
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

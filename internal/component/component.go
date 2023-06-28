package component

import (
	"io"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Exported struct {
	Lock        cargo.BOSHReleaseLock
	TarballPath string
	BlobstoreID string
}

type Local struct {
	Lock      cargo.BOSHReleaseLock
	LocalPath string
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

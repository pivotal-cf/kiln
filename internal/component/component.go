package component

import (
	"io"

	"github.com/pivotal-cf/kiln/pkg/cargo"
)

type Spec = cargo.BOSHReleaseSpec

type Exported struct {
	Lock
	TarballPath string
	BlobstoreID string
}

type Local struct {
	Lock
	LocalPath string
}

type Lock = cargo.BOSHReleaseLock

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

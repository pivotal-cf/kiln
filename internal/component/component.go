package component

import "github.com/pivotal-cf/kiln/pkg/cargo"

type Spec = cargo.ComponentSpec

type Exported struct {
	Lock
	TarballPath string
	BlobstoreID string
}

type Local struct {
	Lock
	LocalPath string
}

type Lock = cargo.ComponentLock

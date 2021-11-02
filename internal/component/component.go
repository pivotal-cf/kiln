package component

import "github.com/pivotal-cf/kiln/pkg/cargo"

type Spec = cargo.ComponentSpec

type Exported struct {
	Spec
	TarballPath string
	BlobstoreID string
	SHA1        string
}

type Local struct {
	Spec
	LocalPath string
	SHA1      string
}

type Lock = cargo.ComponentLock

type Requirement struct {
	Name, Version, VersionConstraint, StemcellOS, StemcellVersion string
}

package cargo

import (
	"github.com/pivotal-cf/kiln/internal/component"
)

type (
	ReleaseSource = component.ReleaseSource

	ReleaseSpec      = component.Spec
	ReleaseLock      = component.Lock
	LocalReleaseLock = component.Local
)

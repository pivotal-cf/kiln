package internal

import "io"

//counterfeiter:generate ./fakes/render_service.go --fake-name RenderService . RenderService
type RenderService interface {
	RenderManifest(tileConfig io.Reader, tileMetadata io.Reader) (string, error)
}

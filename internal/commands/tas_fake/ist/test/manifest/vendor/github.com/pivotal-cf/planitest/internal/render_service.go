package internal

import "io"

//go:generate counterfeiter -o ./fakes/render_service.go --fake-name RenderService . RenderService
type RenderService interface {
	RenderManifest(tileConfig io.Reader, tileMetadata io.Reader) (string, error)
}


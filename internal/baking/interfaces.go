package baking

import (
	"io"

	"github.com/pivotal-cf/kiln/internal/builder"
)

//counterfeiter:generate -o ./fakes/logger.go --fake-name Logger . logger
type logger interface {
	Println(v ...any)
}

//counterfeiter:generate -o ./fakes/part_reader.go --fake-name PartReader . partReader
type partReader interface {
	Read(path string) (builder.Part, error)
}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

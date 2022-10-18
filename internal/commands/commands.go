package commands

import (
	"io"
)

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

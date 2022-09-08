package cargo

import "io"

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

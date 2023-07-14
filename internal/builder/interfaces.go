package builder

import "io"

type logger interface {
	Printf(format string, v ...any)
	Println(v ...any)
}

type Metadata map[string]any

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

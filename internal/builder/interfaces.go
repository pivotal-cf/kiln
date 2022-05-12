package builder

import "io"

type logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type Metadata map[string]interface{}

func closeAndIgnoreError(c io.Closer) { _ = c.Close() }

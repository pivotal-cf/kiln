package builder

import "gopkg.in/yaml.v2"

func init() {
	// Our yaml library altered how it handles line wrapping
	// See: https://github.com/go-yaml/yaml/commit/7649d4548cb53a614db133b2a8ac1f31859dda8c
	yaml.FutureLineWrap()
}

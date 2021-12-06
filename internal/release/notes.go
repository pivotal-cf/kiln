package release

import _ "embed"

//go:embed notes.md.template
var defaultReleaseNotesTemplate string

func DefaultNotesTemplate() string {
	return defaultReleaseNotesTemplate
}

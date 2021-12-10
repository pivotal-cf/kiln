package release

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/masterminds/sprig"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//go:embed notes.md.template
var defaultReleaseNotesTemplate string

func DefaultNotesTemplate() string {
	return defaultReleaseNotesTemplate
}

func DefaultTemplateFuncs(t *template.Template) *template.Template {
	return t.Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap{
		"removeEmptyLines": removeEmptyLines,
	})
}

func removeEmptyLines(input string) string {
	lines := strings.Split(input, "\n")
	var b strings.Builder

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}

		b.WriteString(l)
		b.WriteRune('\n')
	}
	return b.String()
}

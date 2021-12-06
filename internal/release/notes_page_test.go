package release

import (
	_ "embed"
	"io"
	"testing"

	Ω "github.com/onsi/gomega"
)

//go:embed testdata/release_notes.md
var runtimeRNHTML string

func TestParseReleaseNotesPage(t *testing.T) {
	please := Ω.NewWithT(t)

	page, err := ParseNotesPage(runtimeRNHTML)

	please.Expect(err).NotTo(Ω.HaveOccurred())
	please.Expect(page.Prefix).To(Ω.Equal("---\ntitle: TEST\n---\n\n## <a id='releases'></a> Releases"))
	please.Expect(page.Suffix).To(Ω.Equal("## <a id='upgrade'></a>\n\nSome suffix\n"))
	please.Expect(page.Tiles).To(Ω.HaveLen(6))
	please.Expect(page.Tiles[2].Notes).To(Ω.Equal("### <a id='2.7.1'></a> 2.7.1 - Withdrawn\n\nC\n<table>\n<tbody></tbody>\n</table>"))
	please.Expect(page.Tiles[2].Version).To(Ω.Equal("2.7.1"))

	err = page.Tiles[0].Validate()
	please.Expect(err).NotTo(Ω.HaveOccurred())
}

func TestReleaseNotesPage_Add(t *testing.T) {
	t.Fail()
	// TODO:
	// 		test add to empty,
	// 		add to beginning,
	//		add to middle,
	//		replace existing (versions are equal),
	//      existing notes has invalid version (not sure if this will ever happen but meh... maybe good to have somewhat defensive code),
	//		add to end,
	//		maybe add invalid (maybe expect passed TileReleaseNotes to be valid and remove Validate check in Add)
}

func TestReleaseNotesPage_WriteTo(t *testing.T) {
	var _ io.WriterTo = (*NotesPage)(nil)
	// TODO ensure expected output
}

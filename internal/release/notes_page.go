package release

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
)

var releaseNoteExp = regexp.MustCompile(`(?U)(?m)### <a id='(?P<version>\d+\.\d+\.\d+(-.+)?)'></a> (\d+\.\d+\.\d+(-.+)?)\w*(?P<header_suffix>.*)\n((\n.*)*)</tbody>\w*\n\w*</table>`)

type NotesPage struct {
	Prefix, Suffix string
	Tiles          []TileReleaseNotes
}

func ParseNotesPage(input string) (NotesPage, error) {
	const (
		endSentinel   = "## <a id='upgrade'></a>"
		startSentinel = "\n## <a id='releases'></a> Releases"
	)
	end := strings.Index(input, endSentinel)
	start := strings.Index(input, startSentinel) + len(startSentinel)
	if end < 0 || start < 0 {
		panic("start or end sentinel values not found")
	}

	page := NotesPage{
		Prefix: input[:start],
		Suffix: input[end:],
	}

	matchStrings := releaseNoteExp.FindAllStringSubmatch(input, -1)
	versionSubExpIndex := releaseNoteExp.SubexpIndex("version")

	for _, match := range matchStrings {
		page.Tiles = append(page.Tiles, TileReleaseNotes{
			Version: match[versionSubExpIndex],
			Notes:   match[0],
		})
	}

	return page, nil
}

func (page *NotesPage) Add(nt TileReleaseNotes) error {
	err := nt.Validate()
	if err != nil {
		return err
	}

	if len(page.Tiles) == 0 {
		page.Tiles = []TileReleaseNotes{nt}
		return nil
	}

	nv, _ := nt.version()
	for i, t := range page.Tiles {
		tv, err := t.version()
		if err != nil {
			continue
		}
		if nv.Equal(tv) {
			page.Tiles[i] = nt
			return nil
		}
		if !nv.GreaterThan(tv) {
			continue
		}
		page.Tiles = append(page.Tiles[:i], append([]TileReleaseNotes{nt}, page.Tiles[i:]...)...)
		return nil
	}

	page.Tiles = append(page.Tiles, nt)

	return nil
}

func (page *NotesPage) WriteTo(w io.Writer) (n int64, err error) {
	//TODO implement me
	panic("implement me")
}

type TileReleaseNotes struct {
	Version string
	Notes   string
}

func (tile TileReleaseNotes) Validate() error {
	_, err := tile.version()
	if err != nil {
		return fmt.Errorf("invalid version: %w", err)
	}
	if !releaseNoteExp.MatchString(tile.Notes) {
		return fmt.Errorf("notes do not match expression")
	}
	return nil
}

func (tile TileReleaseNotes) version() (*semver.Version, error) {
	return semver.NewVersion(tile.Version)
}

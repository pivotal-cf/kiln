package kiln

import "github.com/pivotal-cf/jhanda/flags"

type ApplicationConfig struct {
	ReleaseTarballs      flags.StringSlice `short:"rt"   long:"release-tarball"         description:""`
	Migrations           flags.StringSlice `short:"m"    long:"migration"               description:""`
	ContentMigrations    flags.StringSlice `short:"cm"   long:"content-migration"       description:""`
	BaseContentMigration string            `short:"bcm"  long:"base-content-migration"  description:""`
	StemcellTarball      string            `short:"st"   long:"stemcell-tarball"        description:""`
	Handcraft            string            `short:"h"    long:"handcraft"               description:""`
	Version              string            `short:"v"    long:"version"                 description:""`
	FinalVersion         string            `short:"fv"   long:"final-version"           description:""`
	ProductName          string            `short:"pn"   long:"product-name"            description:""`
	FilenamePrefix       string            `short:"fp"   long:"filename-prefix"         description:""`
	OutputDir            string            `short:"o"    long:"output-dir"              description:""`
	StubReleases         bool              `short:"sr"   long:"stub-releases"           description:""`
}

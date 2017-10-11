package commands

import "github.com/pivotal-cf/jhanda/flags"

type BakeConfig struct {
	ReleasesDirectory    string            `short:"rd"   long:"releases-directory"      description:"path to the release tarballs directory"`
	MigrationDirectories flags.StringSlice `short:"m"    long:"migrations-directory"    description:"path to the migrations directory"`
	ContentMigrations    flags.StringSlice `short:"cm"   long:"content-migration"       description:"location of the content migration file"`
	BaseContentMigration string            `short:"bcm"  long:"base-content-migration"  description:"location of the base content migration file"`
	StemcellTarball      string            `short:"st"   long:"stemcell-tarball"        description:"location of the stemcell tarball"`
	Handcraft            string            `short:"h"    long:"handcraft"               description:"location of the handcraft file"`
	Version              string            `short:"v"    long:"version"                 description:"version for the filename"`
	FinalVersion         string            `short:"fv"   long:"final-version"           description:"final version of the tile"`
	ProductName          string            `short:"pn"   long:"product-name"            description:"product name"`
	FilenamePrefix       string            `short:"fp"   long:"filename-prefix"         description:"prefix used for filename"`
	OutputDir            string            `short:"o"    long:"output-dir"              description:"output directory"`
	StubReleases         bool              `short:"sr"   long:"stub-releases"           description:"don't include release tarballs"`
}

package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pivotal-cf/jhanda"

	"github.com/pivotal-cf/kiln/internal/builder"
	"github.com/pivotal-cf/kiln/pkg/bake"
)

type ReBake struct {
	bake    jhanda.Command
	Options struct {
		OutputFile string `short:"o" long:"output-file" required:"true" description:"path to where the tile will be output"`
	}
}

func NewReBake(bake jhanda.Command) ReBake {
	return ReBake{bake: bake}
}

func (cmd ReBake) Execute(args []string) error {
	records, err := jhanda.Parse(&cmd.Options, args)
	if err != nil {
		return err
	}
	if len(records) != 1 {
		return fmt.Errorf("please add exactly one required bake record argument: %d bake arguments passed", len(records))
	}
	recordBuffer, err := os.ReadFile(records[0])
	if err != nil {
		return fmt.Errorf("failed to read bake record file: %w", err)
	}

	var record bake.Record
	if err := json.Unmarshal(recordBuffer, &record); err != nil {
		return fmt.Errorf("failed to parse bake record: %w", err)
	}

	workingDirectorySHA, err := builder.GitMetadataSHA(".", false)
	if err != nil {
		return err
	}

	if got, exp := workingDirectorySHA, record.SourceRevision; got != exp {
		return fmt.Errorf("expected the current worktree to be checked out at the source revision from the record %s but the current head is %s", exp, got)
	}

	tileDir := filepath.FromSlash(record.TileDirectory)

	bakeFlags := []string{
		"--version", record.Version,
		"--kilnfile", filepath.Join(tileDir, "Kilnfile"),
		"--output-file", cmd.Options.OutputFile,
	}

	if record.TileName != "" {
		bakeFlags = append(bakeFlags, strings.Join([]string{"--variable", builder.TileNameVariable, record.TileName}, "="))
	}

	if err := cmd.bake.Execute(bakeFlags); err != nil {
		return err
	}

	newRecord, err := bake.NewRecordFromFile(cmd.Options.OutputFile)
	if err != nil {
		return err
	}

	if !record.IsEquivalent(newRecord, log.New(os.Stderr, "bake record diff: ", 0)) {
		return fmt.Errorf("expected tile bake records to be equivilant")
	}

	return nil
}

func (cmd ReBake) Usage() jhanda.Usage {
	return jhanda.Usage{
		Description:      "re-bake (aka record bake) builds a tile from a bake record if the current HEAD is does not match the record the command will fail",
		ShortDescription: "re-bake constructs a tile from a bake record",
		Flags:            &cmd.Options,
	}
}

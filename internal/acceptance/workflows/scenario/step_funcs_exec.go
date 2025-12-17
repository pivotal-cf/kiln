package scenario

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func outputContainsSubstring(ctx context.Context, outputName, substring string) error {
	out, err := output(ctx, outputName)
	if err != nil {
		return err
	}
	outStr := strings.TrimSpace(out.String())
	if s, err := strconv.Unquote(substring); err == nil {
		substring = s
	}
	if !strings.Contains(outStr, substring) {
		if len(outStr) == 0 {
			return fmt.Errorf("expected substring %q not found: %s was empty", substring, outputName)
		}
		if len(outStr) < 5000 {
			return fmt.Errorf("expected substring %q not found in: %q", substring, outStr)
		}
		return fmt.Errorf("expected substring \n\n%s\n\n not found in:\n\n%s", substring, outStr)
	}
	return nil
}

func outputHasSHA256Sum(ctx context.Context, outputName, checksum string) error {
	out, err := output(ctx, outputName)
	if err != nil {
		return fmt.Errorf("failed to get output %q: %w", outputName, err)
	}
	summer := sha256.New()
	_, err = io.Copy(summer, out)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	if sum := hex.EncodeToString(summer.Sum(nil)); sum != checksum {
		return fmt.Errorf("expected checksum %q but got %q", checksum, sum)
	}
	return nil
}

func theExitCodeIs(ctx context.Context, expectedCode int) error {
	state, err := lastCommandProcessState(ctx)
	if err != nil {
		return err
	}
	if state.ExitCode() != expectedCode {
		return fmt.Errorf("expected status code %d but got %d", expectedCode, state.ExitCode())
	}
	return nil
}

func iExecute(ctx context.Context, command string) error {
	args := strings.Fields(command)
	if len(args) < 1 {
		return nil
	}
	dir, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	return executeAndWrapError(dir, nil, args[0], args[1:]...)
}

func iWriteFileWith(ctx context.Context, fileName string, lines *godog.Table) error {
	tileDir, err := tileRepoPath(ctx)
	if err != nil {
		return err
	}
	fileName, err = strconv.Unquote(fileName)
	if err != nil {
		return err
	}

	out := bytes.NewBuffer(nil)
	for i, line := range lines.Rows {
		for _, cell := range line.Cells {
			out.WriteString(cell.Value)
		}
		if i < len(lines.Rows)-1 {
			out.WriteByte('\n')
		}
	}

	fileName = filepath.Join(tileDir, fileName)
	return os.WriteFile(fileName, out.Bytes(), 0o644)
}

package scenario

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

func outputContainsSubstring(ctx context.Context, outputName, substring string) error {
	out, err := output(ctx, outputName)
	if err != nil {
		return err
	}
	buf, err := io.ReadAll(out)
	if err != nil {
		return err
	}
	if !bytes.Contains(buf, []byte(substring)) {
		return fmt.Errorf("expected substring not found in:\n\n%s\n\n", string(buf))
	}
	return err
}

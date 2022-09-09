package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
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
		if len(outStr) < 500 {
			return fmt.Errorf("expected substring %q not found in: %q", substring, outStr)
		}
		return fmt.Errorf("expected substring \n\n%s\n\n not found in:\n\n%s\n\n", substring, outStr)
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

func outputIsValidEncoding(ctx context.Context, outputName, encoding string) error {
	out, err := output(ctx, outputName)
	if err != nil {
		return err
	}
	switch encoding {
	case "json":
		var raw json.RawMessage
		return json.Unmarshal(out.Bytes(), &raw)
	case "yaml":
		var node interface{}
		return yaml.Unmarshal(out.Bytes(), &node)
	default:
		return fmt.Errorf("unknown encoding: %s", encoding)
	}
}

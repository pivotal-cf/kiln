package scenario

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"
)

func runAndLogOnError(ctx context.Context, cmd *exec.Cmd, requireSuccess bool) (context.Context, error) {
	var buf bytes.Buffer
	fds := ctx.Value(standardFileDescriptorsKey).(standardFileDescriptors)
	cmd.Stdout = io.MultiWriter(&buf, fds[1])
	cmd.Stderr = io.MultiWriter(&buf, fds[2])
	runErr := cmd.Run()
	ctx = setLastCommandStatus(ctx, cmd.ProcessState)
	if requireSuccess {
		if runErr != nil {
			_, _ = io.Copy(os.Stdout, &buf)
		}
		return ctx, runErr
	}
	return ctx, nil
}

func runAndParseStdoutAsYAML(ctx context.Context, cmd *exec.Cmd, d interface{}) error {
	var stdout, stderr bytes.Buffer
	fds := ctx.Value(standardFileDescriptorsKey).(standardFileDescriptors)
	cmd.Stdout = io.MultiWriter(&stdout, fds[1])
	cmd.Stderr = io.MultiWriter(&stderr, fds[2])
	runErr := cmd.Run()
	ctx = setLastCommandStatus(ctx, cmd.ProcessState)
	if runErr != nil {
		_, _ = io.Copy(os.Stdout, &stdout)
		return runErr
	}
	return yaml.Unmarshal(stdout.Bytes(), d)
}

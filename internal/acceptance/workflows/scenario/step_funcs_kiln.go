package scenario

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cucumber/godog"
)

func iInvokeKiln(ctx context.Context, table *godog.Table) (context.Context, error) {
	return invokeKiln(ctx, true, argsFromTable(table)...)
}

func iTryToInvokeKiln(ctx context.Context, table *godog.Table) (context.Context, error) {
	return invokeKiln(ctx, false, argsFromTable(table)...)
}

func kilnValidateSucceeds(ctx context.Context) (context.Context, error) {
	return invokeKiln(ctx, true, "validate", "--variable=github_access_token=banana")
}

func invokeKiln(ctx context.Context, requireSuccess bool, args ...string) (context.Context, error) {
	repoPath, err := tileRepoPath(ctx)
	if err != nil {
		return ctx, err
	}

	ex := regexp.MustCompile(`(?m)"\$\{([^]]+)}"`)

	for i, inputArg := range args {
		matches := ex.FindAllStringSubmatch(inputArg, -1)
		for _, match := range matches {
			envName := match[1]
			envPattern := fmt.Sprintf(`"${%s}"`, envName)
			if strings.Contains(inputArg, envPattern) {
				args[i] = strings.Replace(inputArg, envPattern, os.Getenv(envName), -1)
			}
		}
	}

	cmd := kilnCommand(ctx, args...)
	cmd.Dir = repoPath

	return runAndLogOnError(ctx, cmd, requireSuccess)
}

func argsFromTable(table *godog.Table) []string {
	var result []string
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			result = append(result, cell.Value)
		}
	}
	return result
}

package scenario

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

func hasRegexMatches(ctx context.Context, outputName, expression string, table *godog.Table) error {
	ex, err := regexp.Compile(expression)
	if err != nil {
		return err
	}

	buf, err := output(ctx, outputName)
	if err != nil {
		return err
	}

	err = ensureUniqueColumnHeadersExistForAllSubExpressionNames(table, ex)
	if err != nil {
		return err
	}

	allSubMatches := ex.FindAllSubmatch(buf.Bytes(), -1)

	expLen := len(table.Rows) - 1
	gotLen := len(allSubMatches)
	if expLen != gotLen {
		return fmt.Errorf("expected %d matches but got %d", expLen, gotLen)
	}

	columnNamesToColumnIndex := make(map[string]int)
	for index, cell := range table.Rows[0].Cells {
		columnNamesToColumnIndex[cell.Value] = index
	}

	var errBuilder strings.Builder
	subExpNames := ex.SubexpNames()
	for rowIndex, gotMatch := range allSubMatches {
		expectedRow := table.Rows[rowIndex+1]

		for matchIndex, matchBytes := range gotMatch[1:] {
			gotString := string(matchBytes)
			matchName := subExpNames[matchIndex+1]
			columnIndex := columnNamesToColumnIndex[matchName]

			expString := expectedRow.Cells[columnIndex].Value

			if unQuoted, err := strconv.Unquote(expString); err == nil {
				expString = unQuoted
			}

			if gotString != expString {
				errBuilder.WriteString(fmt.Sprintf("expected match %d submatch named %s to equal %q but got %q\n", rowIndex, matchName, expString, gotString))
			}
		}
	}

	if l := errBuilder.Len(); l > 0 {
		return errors.New(errBuilder.String()[:l-1])
	}

	return nil
}

func ensureUniqueColumnHeadersExistForAllSubExpressionNames(table *godog.Table, exp *regexp.Regexp) error {
	set := make(map[string]struct{})
	if len(table.Rows) > 0 {
		for _, cell := range table.Rows[0].Cells {
			set[cell.Value] = struct{}{}
		}
	}
	var namesNotFound []string
	for _, name := range exp.SubexpNames() {
		_, found := set[name]
		if !found && name != "" {
			namesNotFound = append(namesNotFound, name)
		}
	}
	if len(namesNotFound) > 0 {
		return fmt.Errorf("expected first row to contain the names of sub expressions: missing %v", namesNotFound)
	}
	columnNamesToIndex := make(map[string]int)
	for index, cell := range table.Rows[0].Cells {
		foundIndex, found := columnNamesToIndex[cell.Value]
		if found {
			return fmt.Errorf("column name %q is not unique in the table, column %d has the same name", cell.Value, foundIndex)
		}
		columnNamesToIndex[cell.Value] = index
	}
	return nil
}

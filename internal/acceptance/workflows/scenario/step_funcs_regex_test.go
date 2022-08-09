package scenario

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/cucumber/godog"
	"github.com/cucumber/messages-go/v16"
)

func Test_hasRegexMatches(t *testing.T) {
	testHappyPath := func(input, expression string, expected *godog.Table) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			ctx := context.Background()
			ctx = context.WithValue(ctx, standardFileDescriptorsKey, standardFileDescriptors{
				nil,
				bytes.NewBuffer([]byte(input)),
				nil,
			})

			err := hasRegexMatches(ctx, "stdout", expression, expected)

			if err != nil {
				t.Errorf("unexpected error: %s", err)
			}
		}
	}
	testExpectedError := func(expectedError, input, expression string, expected *godog.Table) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			ctx := context.Background()
			ctx = context.WithValue(ctx, standardFileDescriptorsKey, standardFileDescriptors{
				nil,
				bytes.NewBuffer([]byte(input)),
				nil,
			})

			err := hasRegexMatches(ctx, "stdout", expression, expected)

			if err == nil {
				t.Errorf("unexpected error got nil")
				return
			}
			if gotMessage := err.Error(); gotMessage != expectedError {
				t.Errorf("err and expected err do not match\nexp: %s\n\ngot: %s", gotMessage, expectedError)
			}
		}
	}

	someTestExpression := regexp.MustCompile(`\s+(?P<name>[a-z]+):\s*(?P<number>\d+)`)

	t.Run("all matches are found", testHappyPath(`
one: 1
two: 2
   three:3
something
hello
four:    4
`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"}, {Value: "number"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "one"}, {Value: "1"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "two"}, {Value: "2"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "three"}, {Value: "3"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "four"}, {Value: "4"},
				}},
			},
		},
	))

	t.Run("expected values are quoted", testHappyPath(`
one: 1
`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"}, {Value: "number"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: `"one"`}, {Value: "1"},
				}},
			},
		},
	))

	t.Run("expected values are not quoted and have surrounding whitespace", testHappyPath(`
one: 1
`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"}, {Value: "number"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "one"}, {Value: "1"},
				}},
			},
		},
	))

	t.Run("duplicate column definition", testHappyPath(`
a	a
b	b
c	c
`,
		`(?P<c>\w+)\t(?P<c>\w+)`,
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "c"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "a"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "b"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "c"},
				}},
			},
		},
	))

	t.Run("no matches are found", testExpectedError(
		"expected 1 matches but got 0",
		`banana`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"}, {Value: "number"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "one"}, {Value: "1"},
				}},
			},
		},
	))

	t.Run("an unexpected match is found", testExpectedError(
		"expected 0 matches but got 1",
		`
one: 1
`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"}, {Value: "number"},
				}},
			},
		},
	))

	t.Run("missing column definition", testExpectedError(
		"expected first row to contain the names of sub expressions: missing [number]",
		`
one: 1
`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{
				{Cells: []*messages.PickleTableCell{
					{Value: "name"},
				}},
				{Cells: []*messages.PickleTableCell{
					{Value: "one"},
				}},
			},
		},
	))

	t.Run("missing table header", testExpectedError(
		"expected first row to contain the names of sub expressions: missing [name number]",
		`one: 1`,
		someTestExpression.String(),
		&godog.Table{
			Rows: []*messages.PickleTableRow{},
		},
	))
}

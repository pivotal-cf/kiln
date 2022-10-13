package commands

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_extractMajorVersion(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult string
		expectErrTo    OmegaMatcher
	}{
		{
			name:           "with tilde ~",
			input:          "~456",
			expectedResult: "456",
			expectErrTo:    Not(HaveOccurred()),
		},
		{
			name:           "with hypens -",
			input:          "777.1-621",
			expectedResult: "777",
			expectErrTo:    Not(HaveOccurred()),
		},
		{
			name:           "with wildcards *",
			input:          "1234.*",
			expectedResult: "1234",
			expectErrTo:    Not(HaveOccurred()),
		},
		{
			name:           "with caret ^",
			input:          "^456",
			expectedResult: "456",
			expectErrTo:    Not(HaveOccurred()),
		},

		{
			name:           "with absolute value",
			input:          "333.334",
			expectedResult: "333",
			expectErrTo:    Not(HaveOccurred()),
		},
		{
			name:           "specifier does not have major version",
			input:          "*",
			expectedResult: "",
			expectErrTo: And(
				HaveOccurred(),
				MatchError(ContainSubstring(ErrStemcellMajorVersionMustBeValid)),
			),
		},
		{
			name:           "specifier is an empty string",
			input:          "",
			expectedResult: "",
			expectErrTo: And(
				HaveOccurred(),
				MatchError(ContainSubstring(ErrStemcellMajorVersionMustBeValid)),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			please := NewWithT(t)

			got, err := extractMajorVersion(tt.input)
			please.Expect(err).To(tt.expectErrTo)
			please.Expect(got).To(Equal(tt.expectedResult))
		})
	}
}

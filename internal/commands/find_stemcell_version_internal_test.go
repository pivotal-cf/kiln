package commands

import (
	"testing"

	Ω "github.com/onsi/gomega"
)

func Test_extractMajorVersion(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedResult string
		expectErrTo    Ω.OmegaMatcher
	}{
		{name: "with tilde ~",
			input:          "~456",
			expectedResult: "456",
			expectErrTo:    Ω.Not(Ω.HaveOccurred()),
		},
		{name: "with hypens -",
			input:          "777.1-621",
			expectedResult: "777",
			expectErrTo:    Ω.Not(Ω.HaveOccurred()),
		},
		{name: "with wildcards *",
			input:          "1234.*",
			expectedResult: "1234",
			expectErrTo:    Ω.Not(Ω.HaveOccurred()),
		},
		{name: "with caret ^",
			input:          "^456",
			expectedResult: "456",
			expectErrTo:    Ω.Not(Ω.HaveOccurred()),
		},

		{name: "with absolute value",
			input:          "333.334",
			expectedResult: "333",
			expectErrTo:    Ω.Not(Ω.HaveOccurred()),
		},
		{name: "specifier does not have major version",
			input:          "*",
			expectedResult: "",
			expectErrTo: Ω.And(
				Ω.HaveOccurred(),
				Ω.MatchError(Ω.ContainSubstring(ErrStemcellMajorVersionMustBeValid)),
			),
		},
		{name: "specifier is an empty string",
			input:          "",
			expectedResult: "",
			expectErrTo: Ω.And(
				Ω.HaveOccurred(),
				Ω.MatchError(Ω.ContainSubstring(ErrStemcellMajorVersionMustBeValid)),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			please := Ω.NewWithT(t)

			got, err := extractMajorVersion(tt.input)
			please.Expect(err).To(tt.expectErrTo)
			please.Expect(got).To(Ω.Equal(tt.expectedResult))
		})
	}
}

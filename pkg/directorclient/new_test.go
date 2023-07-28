package directorclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_overrideEnvironmentWithConfigStructure(t *testing.T) {
	t.Run("when the configStructure is not a struct", func(t *testing.T) {
		type T map[string]any

		assert.Panics(t, func() {
			overrideEnvironmentWithConfigurationStructure(T{}, []string{
				"MELON=watermelon",
			})
		})
	})

	for _, tt := range []struct {
		Name            string
		config          any
		environ         []string
		expectedEnviron []string
	}{
		{
			Name: "when the name-value pairs do not exist in the input slice",
			config: struct {
				Apple string `env:"APPLE"`
				Mango string `env:"MANGO"`
			}{
				Apple: "McIntosh",
				Mango: "Alphonso",
			},
			environ:         []string{"MELON=watermelon"},
			expectedEnviron: []string{"MELON=watermelon", "APPLE=McIntosh", "MANGO=Alphonso"},
		},
		{
			Name: "when the struct field is not a string",
			config: struct {
				Orange int `env:"ORANGE"`
			}{
				Orange: 5, // the value does not matter
			},
			environ:         []string{"MELON=watermelon"},
			expectedEnviron: []string{"MELON=watermelon"},
		},
		{
			Name: "when the struct field is private",
			config: struct {
				orange string `env:"ORANGE"`
			}{
				orange: "Valencia",
			},
			environ:         []string{"MELON=watermelon"},
			expectedEnviron: []string{"MELON=watermelon"},
		},
		{
			Name: "when the env tag is not set",
			config: struct {
				Orange string
			}{
				Orange: "Naval",
			},
			environ:         []string{"MELON=watermelon"},
			expectedEnviron: []string{"MELON=watermelon"},
		},
		{
			Name: "when environ contains a variable with the same name",
			config: struct {
				Orange string `env:"ORANGE"`
			}{
				Orange: "Naval",
			},
			environ:         []string{"ORANGE=Valencia"},
			expectedEnviron: []string{"ORANGE=Naval"},
		},
	} {
		t.Run(tt.Name, func(t *testing.T) {
			require.NotPanics(t, func() {
				result := overrideEnvironmentWithConfigurationStructure(tt.config, tt.environ)
				assert.Equal(t, tt.expectedEnviron, result)
			})
		})
	}
}

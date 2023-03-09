package cargo

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v2"

	"github.com/pivotal-cf/kiln/internal/builder"
)

func InterpolateAndParseKilnfile(in io.Reader, templateVariables map[string]interface{}) (Kilnfile, error) {
	kilnfileYAML, err := io.ReadAll(in)
	if err != nil {
		return Kilnfile{}, fmt.Errorf("unable to read Kilnfile: %w", err)
	}

	interpolator := builder.NewInterpolator()
	interpolatedMetadata, err := interpolator.Interpolate(builder.InterpolateInput{
		Variables: templateVariables,
	}, KilnfileFileName, kilnfileYAML)
	if err != nil {
		return Kilnfile{}, err
	}

	var kilnfile Kilnfile
	err = yaml.Unmarshal(interpolatedMetadata, &kilnfile)
	if err != nil {
		return Kilnfile{}, err
	}

	return kilnfile, nil
}

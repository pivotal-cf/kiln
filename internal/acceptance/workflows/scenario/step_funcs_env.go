package scenario

import (
	"fmt"
	"os"
	"strings"
)

func theEnvironmentVariableIsSet(name string) error {
	prefix := name + "="
	for _, v := range os.Environ() {
		if strings.HasPrefix(v, prefix) {
			return nil
		}
	}
	return fmt.Errorf("environment variable %s not set", name)
}

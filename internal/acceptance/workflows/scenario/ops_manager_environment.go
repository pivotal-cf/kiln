package scenario

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

type opsManagerEnvironment struct {
	OpsManagerPrivateKey string
	OpsManager           struct {
		URL      string
		Password string
		Username string
	}
	AvailabilityZones []string
	ServiceSubnetName string
}

func (env *opsManagerEnvironment) loadFromEnvironmentVariables() error {
	var err error
	env.OpsManager.URL, err = loadEnvironmentVariable("OM_TARGET", "")
	if err != nil {
		return err
	}
	env.OpsManager.Username, err = loadEnvironmentVariable("OM_USERNAME", "")
	if err != nil {
		return err
	}
	env.OpsManager.Password, err = loadEnvironmentVariable("OM_PASSWORD", "")
	if err != nil {
		return err
	}
	boshAllProxy, err := loadEnvironmentVariable("BOSH_ALL_PROXY", "")
	if err != nil {
		return err
	}
	_, loadOmPrivateKeyErr := loadEnvironmentVariable("OM_PRIVATE_KEY", "")
	if loadOmPrivateKeyErr != nil {
		privateKey, err := readPrivateKeyFromBOSHAllProxyURL(boshAllProxy)
		if err != nil {
			return err
		}
		err = os.Setenv("OM_PRIVATE_KEY", privateKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (env *opsManagerEnvironment) fetchNetworksAndAvailabilityZones(ctx context.Context) (context.Context, error) {
	var directorConfig struct {
		AvailabilityZoneConfiguration []struct {
			Name string `yaml:"name"`
		} `yaml:"az-configuration"`
		NetworkConfiguration struct {
			Networks []struct {
				Name string `yaml:"name"`
			} `yaml:"networks"`
		} `yaml:"networks-configuration"`
	}
	ctx, err := runAndParseStdoutAsYAML(ctx,
		exec.Command("om", "--skip-ssl-validation", "staged-director-config", "--no-redact"),
		&directorConfig,
	)
	if err != nil {
		return ctx, err
	}
	for _, az := range directorConfig.AvailabilityZoneConfiguration {
		env.AvailabilityZones = append(env.AvailabilityZones, az.Name)
	}
	for _, network := range directorConfig.NetworkConfiguration.Networks {
		if !strings.HasSuffix(network.Name, "-services-subnet") {
			continue
		}
		env.ServiceSubnetName = network.Name
		break
	}
	return ctx, err
}

func readPrivateKeyFromBOSHAllProxyURL(boshAllProxy string) (string, error) {
	const failedToSetOMPrivateKey = "failed to set OM_PRIVATE_KEY from BOSH_ALL_PROXY: %w"
	u, err := url.Parse(boshAllProxy)
	if err != nil {
		return "", fmt.Errorf(failedToSetOMPrivateKey, err)
	}
	keyPath := u.Query().Get("private-key")
	if keyPath == "" {
		return "", fmt.Errorf(failedToSetOMPrivateKey, fmt.Errorf(`url did not have "private-key"`))
	}
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf(failedToSetOMPrivateKey, err)
	}
	return string(keyBytes), nil
}

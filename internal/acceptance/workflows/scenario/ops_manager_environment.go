package scenario

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type opsManagerEnvironment struct {
	OpsManager struct {
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
	_, err = loadEnvironmentVariable("BOSH_ALL_PROXY", "")
	if err != nil {
		return err
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

func fetchAssociatedStemcellVersion(ctx context.Context, productID string) (string, error) {
	var stemcellAssociations struct {
		Products []struct {
			ID                string `yaml:"identifier"`
			DeployedStemcells []struct {
				Version string `yaml:"version"`
			} `yaml:"deployed_stemcells"`
		} `yaml:"products"`
	}
	var err error
	_, err = runAndParseStdoutAsYAML(ctx,
		exec.Command("om", "curl", "--silent", "--path", "/api/v0/stemcell_associations"),
		&stemcellAssociations,
	)
	if err != nil {
		return "", err
	}
	for _, p := range stemcellAssociations.Products {
		if p.ID != productID {
			continue
		}
		for _, s := range p.DeployedStemcells {
			return s.Version, nil
		}
	}
	return "", fmt.Errorf("no stemcells found on ops manager")
}

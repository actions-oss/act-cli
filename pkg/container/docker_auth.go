//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"

	"github.com/actions-oss/act-cli/pkg/common"
	"github.com/distribution/reference"
	"github.com/docker/cli/cli/config"
	"github.com/moby/moby/api/types/registry"
)

func LoadDockerAuthConfig(ctx context.Context, image string) (registry.AuthConfig, error) {
	logger := common.Logger(ctx)
	cfg := config.LoadDefaultConfigFile(nil)

	registryKey := registryAuthConfigKey("docker.io")
	if image != "" {
		registryRef, err := reference.ParseNormalizedNamed(image)
		if err != nil {
			logger.Warnf("Could not normalize image reference: %v", err)
			return registry.AuthConfig{}, nil
		}
		registryKey = registryAuthConfigKey(reference.Domain(registryRef))
	}

	authConfig, err := cfg.GetAuthConfig(registryKey)
	if err != nil {
		logger.Warnf("Could not get auth config from docker config: %v", err)
		return registry.AuthConfig{}, nil
	}

	return registry.AuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		Auth:          authConfig.Auth,
		ServerAddress: authConfig.ServerAddress,
		IdentityToken: authConfig.IdentityToken,
		RegistryToken: authConfig.RegistryToken,
	}, nil
}

func LoadDockerAuthConfigs(ctx context.Context) map[string]registry.AuthConfig {
	logger := common.Logger(ctx)
	cfg := config.LoadDefaultConfigFile(nil)

	creds, err := cfg.GetAllCredentials()
	if err != nil {
		logger.Warnf("Could not get docker auth configs: %v", err)
		return nil
	}
	authConfigs := make(map[string]registry.AuthConfig, len(creds))
	for k, v := range creds {
		authConfigs[k] = registry.AuthConfig{
			Username:      v.Username,
			Password:      v.Password,
			Auth:          v.Auth,
			ServerAddress: v.ServerAddress,
			IdentityToken: v.IdentityToken,
			RegistryToken: v.RegistryToken,
		}
	}

	return authConfigs
}

func registryAuthConfigKey(domainName string) string {
	if domainName == "docker.io" || domainName == "index.docker.io" {
		return "https://index.docker.io/v1/"
	}
	return domainName
}

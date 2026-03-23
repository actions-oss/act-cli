package container

import (
	"context"
	"testing"

	"github.com/docker/cli/cli/config"
	"github.com/moby/moby/api/pkg/authconfig"
	specs "github.com/opencontainers/image-spec/specs-go/v1"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestCleanImage(t *testing.T) {
	tables := []struct {
		imageIn  string
		imageOut string
	}{
		{"myhost.com/foo/bar", "myhost.com/foo/bar"},
		{"localhost:8000/canonical/ubuntu", "localhost:8000/canonical/ubuntu"},
		{"localhost/canonical/ubuntu:latest", "localhost/canonical/ubuntu:latest"},
		{"localhost:8000/canonical/ubuntu:latest", "localhost:8000/canonical/ubuntu:latest"},
		{"ubuntu", "docker.io/library/ubuntu"},
		{"ubuntu:18.04", "docker.io/library/ubuntu:18.04"},
		{"cibuilds/hugo:0.53", "docker.io/cibuilds/hugo:0.53"},
	}

	for _, table := range tables {
		imageOut := cleanImage(context.Background(), table.imageIn)
		assert.Equal(t, table.imageOut, imageOut)
	}
}

func TestGetImagePullOptions(t *testing.T) {
	ctx := context.Background()
	originalDir := config.Dir()
	t.Cleanup(func() {
		config.SetDir(originalDir)
	})

	config.SetDir("/non-existent/docker")
	t.Setenv("DOCKER_CONFIG", "/non-existent/docker")

	options, err := getImagePullOptions(ctx, NewDockerPullExecutorInput{})
	assert.NoError(t, err, "Failed to create ImagePullOptions")
	assert.Empty(t, options.RegistryAuth, "RegistryAuth should be empty if no username or password is set")

	options, err = getImagePullOptions(ctx, NewDockerPullExecutorInput{
		Image:    "alpine:latest",
		Platform: "linux/amd64",
		Username: "username",
		Password: "password",
	})
	assert.NoError(t, err, "Failed to create ImagePullOptions")
	assert.Equal(t, []specs.Platform{{OS: "linux", Architecture: "amd64"}}, options.Platforms)
	explicitAuth, err := authconfig.Decode(options.RegistryAuth)
	assert.NoError(t, err)
	assert.Equal(t, "username", explicitAuth.Username)
	assert.Equal(t, "password", explicitAuth.Password)

	config.SetDir("testdata/docker-pull-options")

	options, err = getImagePullOptions(ctx, NewDockerPullExecutorInput{
		Image: "nektos/act",
	})
	assert.NoError(t, err, "Failed to create ImagePullOptions")
	dockerAuth, err := authconfig.Decode(options.RegistryAuth)
	assert.NoError(t, err)
	assert.Equal(t, "username", dockerAuth.Username)
	assert.Equal(t, "password\n", dockerAuth.Password)
	assert.Equal(t, "https://index.docker.io/v1/", dockerAuth.ServerAddress)
}

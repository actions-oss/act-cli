package container

import (
	"context"
	"errors"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func stubDockerHostProbe(t *testing.T, probe func(context.Context, string) error) {
	t.Helper()

	originalProbe := dockerHostProbe
	dockerHostProbe = probe
	t.Cleanup(func() {
		dockerHostProbe = originalProbe
	})
}

func stubCommonSocketLocations(t *testing.T, locations []string) {
	t.Helper()

	originalLocations := CommonSocketLocations
	CommonSocketLocations = locations
	t.Cleanup(func() {
		CommonSocketLocations = originalLocations
	})
}

func createSocketCandidate(t *testing.T) string {
	t.Helper()

	socketFile, err := os.CreateTemp("", "act-*.sock")
	assert.NoError(t, err)
	assert.NoError(t, socketFile.Close())
	t.Cleanup(func() {
		_ = os.Remove(socketFile.Name())
	})

	return socketFile.Name()
}

func TestGetSocketAndHostUsesReachableDockerHost(t *testing.T) {
	dockerHost := "unix:///my/docker/host.sock"
	t.Setenv("DOCKER_HOST", dockerHost)
	stubCommonSocketLocations(t, nil)
	stubDockerHostProbe(t, func(_ context.Context, host string) error {
		if host == dockerHost {
			return nil
		}
		return errors.New("unexpected host")
	})

	ret, err := GetSocketAndHost("")

	assert.NoError(t, err)
	assert.Equal(t, SocketAndHost{Socket: dockerHost, Host: dockerHost}, ret)
}

func TestGetSocketAndHostErrorsWhenDockerHostUnreachable(t *testing.T) {
	dockerHost := "unix:///my/docker/host.sock"
	t.Setenv("DOCKER_HOST", dockerHost)
	stubCommonSocketLocations(t, nil)
	stubDockerHostProbe(t, func(_ context.Context, host string) error {
		if host == dockerHost {
			return errors.New("unreachable")
		}
		return nil
	})

	ret, err := GetSocketAndHost("")

	assert.Equal(t, SocketAndHost{}, ret)
	assert.ErrorContains(t, err, `DOCKER_HOST "unix:///my/docker/host.sock" is unreachable`)
}

func TestGetSocketAndHostFallsBackToReachableDefaultRuntime(t *testing.T) {
	dockerSocket := createSocketCandidate(t)
	podmanSocket := createSocketCandidate(t)
	podmanHost := "unix://" + podmanSocket
	t.Setenv("DOCKER_HOST", "")
	stubCommonSocketLocations(t, []string{dockerSocket, podmanSocket})
	stubDockerHostProbe(t, func(_ context.Context, host string) error {
		switch host {
		case "unix://" + dockerSocket:
			return errors.New("docker socket is down")
		case podmanHost:
			return nil
		default:
			return errors.New("unexpected host")
		}
	})

	ret, err := GetSocketAndHost("")

	assert.NoError(t, err)
	assert.Equal(t, SocketAndHost{Socket: podmanHost, Host: podmanHost}, ret)
}

func TestGetSocketAndHostErrorsWhenNoRuntimeReachable(t *testing.T) {
	socket := createSocketCandidate(t)
	t.Setenv("DOCKER_HOST", "")
	stubCommonSocketLocations(t, []string{socket})
	stubDockerHostProbe(t, func(_ context.Context, _ string) error {
		return errors.New("unreachable")
	})

	ret, err := GetSocketAndHost("")

	assert.Equal(t, SocketAndHost{}, ret)
	assert.ErrorContains(t, err, "no reachable container runtime found")
}

func TestGetSocketAndHostUsesReachableExplicitSocket(t *testing.T) {
	socketURI := "unix:///path/to/my.socket"
	t.Setenv("DOCKER_HOST", "")
	stubCommonSocketLocations(t, nil)
	stubDockerHostProbe(t, func(_ context.Context, host string) error {
		if host == socketURI {
			return nil
		}
		return errors.New("unexpected host")
	})

	ret, err := GetSocketAndHost(socketURI)

	assert.NoError(t, err)
	assert.Equal(t, SocketAndHost{Socket: socketURI, Host: socketURI}, ret)
}

func TestGetSocketAndHostRejectsInvalidSocketWithoutRuntime(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	stubCommonSocketLocations(t, nil)
	stubDockerHostProbe(t, func(_ context.Context, _ string) error {
		return errors.New("unreachable")
	})

	ret, err := GetSocketAndHost("/path/to/my.socket")

	assert.Equal(t, SocketAndHost{}, ret)
	assert.ErrorContains(t, err, "container daemon socket ('/path/to/my.socket') is invalid")
}

func TestGetSocketAndHostPreservesDashSocketWhenDockerHostSet(t *testing.T) {
	dockerHost := "unix:///my/docker/host.sock"
	t.Setenv("DOCKER_HOST", dockerHost)
	stubCommonSocketLocations(t, nil)
	stubDockerHostProbe(t, func(_ context.Context, host string) error {
		if host == dockerHost {
			return nil
		}
		return errors.New("unexpected host")
	})

	ret, err := GetSocketAndHost("-")

	assert.NoError(t, err)
	assert.Equal(t, SocketAndHost{Socket: "-", Host: dockerHost}, ret)
}

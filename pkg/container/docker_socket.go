//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var CommonSocketLocations = []string{
	"/var/run/docker.sock",
	"/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	"$XDG_RUNTIME_DIR/podman/podman.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

var dockerHostProbe = pingDockerHost

func socketCandidates() []string {
	candidates := make([]string, 0, len(CommonSocketLocations))
	for _, p := range CommonSocketLocations {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				candidates = append(candidates, "npipe://"+filepath.ToSlash(os.ExpandEnv(p)))
				continue
			}
			candidates = append(candidates, "unix://"+filepath.ToSlash(os.ExpandEnv(p)))
		}
	}
	return candidates
}

// This function, `isDockerHostURI`, takes a string argument `daemonPath`. It checks if the
// `daemonPath` is a valid Docker host URI. It does this by checking if the scheme of the URI (the
// part before "://") contains only alphabetic characters. If it does, the function returns true,
// indicating that the `daemonPath` is a Docker host URI. If it doesn't, or if the "://" delimiter
// is not found in the `daemonPath`, the function returns false.
func isDockerHostURI(daemonPath string) bool {
	if protoIndex := strings.Index(daemonPath, "://"); protoIndex != -1 {
		scheme := daemonPath[:protoIndex]
		if strings.IndexFunc(scheme, func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z')
		}) == -1 {
			return true
		}
	}
	return false
}

type SocketAndHost struct {
	Socket string
	Host   string
}

func resolveReachableDockerHost(ctx context.Context) (string, error) {
	if dockerHost, exists := os.LookupEnv("DOCKER_HOST"); exists && dockerHost != "" {
		if err := probeDockerHost(ctx, dockerHost); err != nil {
			return "", fmt.Errorf("docker host aka DOCKER_HOST %q is unreachable: %w", dockerHost, err)
		}
		return dockerHost, nil
	}

	for _, candidate := range socketCandidates() {
		if err := probeDockerHost(ctx, candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no reachable container runtime found in the usual locations")
}

func probeDockerHost(ctx context.Context, host string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return dockerHostProbe(ctx, host)
}

func GetSocketAndHost(containerSocket string) (SocketAndHost, error) {
	log.Debugf("Handling container host and socket")

	ctx := context.Background()

	// ** socketHost.Socket cases **
	// Case 1: User does _not_ want to mount a daemon socket (passes a dash)
	// Case 2: User passes a filepath to the socket; is that even valid?
	// Case 3: User passes a valid socket; do nothing
	// Case 4: User omitted the flag; set a sane default

	// ** DOCKER_HOST cases **
	// Case A: DOCKER_HOST is set; use it, i.e. do nothing
	// Case B: DOCKER_HOST is empty; use sane defaults

	// Case 3B: User supplied a valid URI socket. Probe it directly before
	// doing the more expensive generic resolution, since we already have
	// a specific target.
	if containerSocket != "" && containerSocket != "-" && isDockerHostURI(containerSocket) {
		if err := probeDockerHost(ctx, containerSocket); err != nil {
			return SocketAndHost{}, fmt.Errorf("container daemon socket %q is unreachable: %w", containerSocket, err)
		}
		log.Debugf("Setting DOCKER_HOST to container socket '%s'", containerSocket)
		return SocketAndHost{Socket: containerSocket, Host: containerSocket}, nil
	}

	// Resolve a reachable docker host (checks DOCKER_HOST, then common socket locations)
	dockerHost, resolveErr := resolveReachableDockerHost(ctx)
	hasDockerHost := resolveErr == nil

	socketHost := SocketAndHost{Socket: containerSocket, Host: dockerHost}

	// If no runtime found and socket is empty or dash, fail early
	if !hasDockerHost && (socketHost.Socket == "" || socketHost.Socket == "-") {
		return SocketAndHost{}, resolveErr
	}

	// A - (dash) in socketHost.Socket means don't mount, preserve this value
	// otherwise if socketHost.Socket is a filepath don't use it as socket
	// Exit early if we're in an invalid state (e.g. when no DOCKER_HOST and user supplied "-", a dash or omitted)
	if !hasDockerHost && socketHost.Socket != "" && !isDockerHostURI(socketHost.Socket) {
		// Cases: 1B, 2B
		return SocketAndHost{}, fmt.Errorf("docker host aka DOCKER_HOST was not set, couldn't be found in the usual locations, and the container daemon socket ('%s') is invalid", socketHost.Socket)
	}

	// Default to DOCKER_HOST if set
	if socketHost.Socket == "" && hasDockerHost {
		// Cases: 4A
		log.Debugf("Defaulting container socket to DOCKER_HOST")
		socketHost.Socket = socketHost.Host
	}
	// Set sane default socket location if user omitted it
	if socketHost.Socket == "" {
		// Cases: 4B
		log.Debugf("Defaulting container socket to default '%s'", socketHost.Host)
		socketHost.Socket = socketHost.Host
	}

	// Exit if both the DOCKER_HOST and socket are fulfilled
	if hasDockerHost {
		// Cases: 1A, 2A, 3A, 4A
		if !isDockerHostURI(socketHost.Socket) {
			// Cases: 1A, 2A
			log.Debugf("DOCKER_HOST is set, but socket is invalid '%s'", socketHost.Socket)
		}
		return socketHost, nil
	}

	return SocketAndHost{}, fmt.Errorf("no DOCKER_HOST and an invalid container socket '%s'", socketHost.Socket)
}

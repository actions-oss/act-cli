//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func parsePlatform(platform string) *specs.Platform {
	if platform == "" {
		return nil
	}

	parts := strings.Split(platform, "/")
	if len(parts) < 2 {
		return nil
	}

	spec := &specs.Platform{
		OS:           parts[0],
		Architecture: parts[1],
	}
	if len(parts) > 2 {
		spec.Variant = parts[2]
	}

	return spec
}

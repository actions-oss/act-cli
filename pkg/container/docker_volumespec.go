//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"errors"
	"fmt"

	mounttypes "github.com/moby/moby/api/types/mount"
)

const endOfVolumeSpec = rune(0)

// volumeSpec is a local copy of the small subset of docker/cli's volume parser
// that we need when translating `-v/--volume` flags into binds vs volumes.
type volumeSpec struct {
	Type     string
	Source   string
	Target   string
	ReadOnly bool
}

func parseVolumeSpec(spec string) (volumeSpec, error) {
	volume := volumeSpec{}

	switch len(spec) {
	case 0:
		return volume, errors.New("invalid empty volume spec")
	case 1, 2:
		volume.Target = spec
		volume.Type = string(mounttypes.TypeVolume)
		return volume, nil
	}

	buffer := []rune{}
	for _, char := range spec + string(endOfVolumeSpec) {
		switch {
		case isWindowsDrive(buffer, char):
			buffer = append(buffer, char)
		case char == ':' || char == endOfVolumeSpec:
			if err := populateVolumeField(char, buffer, &volume); err != nil {
				populateVolumeType(&volume)
				return volume, fmt.Errorf("invalid spec: %s: %w", spec, err)
			}
			buffer = []rune{}
		default:
			buffer = append(buffer, char)
		}
	}

	populateVolumeType(&volume)
	return volume, nil
}

func isWindowsDrive(buffer []rune, char rune) bool {
	return char == ':' && len(buffer) == 1 && unicode.IsLetter(buffer[0])
}

func populateVolumeField(char rune, buffer []rune, volume *volumeSpec) error {
	strBuffer := string(buffer)
	switch {
	case len(buffer) == 0:
		return errors.New("empty section between colons")
	case volume.Source == "" && char == endOfVolumeSpec:
		volume.Target = strBuffer
		return nil
	case volume.Source == "":
		volume.Source = strBuffer
		return nil
	case volume.Target == "":
		volume.Target = strBuffer
		return nil
	case char == ':':
		return errors.New("too many colons")
	}
	for _, option := range strings.Split(strBuffer, ",") {
		switch option {
		case "ro":
			volume.ReadOnly = true
		case "rw":
			volume.ReadOnly = false
		}
	}
	return nil
}

func populateVolumeType(volume *volumeSpec) {
	switch {
	case volume.Source == "":
		volume.Type = string(mounttypes.TypeVolume)
	case isFilePath(volume.Source):
		volume.Type = string(mounttypes.TypeBind)
	default:
		volume.Type = string(mounttypes.TypeVolume)
	}
}

func isFilePath(source string) bool {
	switch source[0] {
	case '.', '/', '~':
		return true
	}
	if len([]rune(source)) == 1 {
		return false
	}
	if strings.HasPrefix(source, `\\`) {
		return true
	}

	first, nextIndex := utf8.DecodeRuneInString(source)
	return isWindowsDrive([]rune{first}, rune(source[nextIndex]))
}

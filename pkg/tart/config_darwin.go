package tart

import "io"

type Config struct {
	SSHUsername string
	SSHPassword string
	Softnet     bool
	Headless    bool
	AlwaysPull  bool
	Writer      io.Writer
}

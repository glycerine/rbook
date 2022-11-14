package main

import (
	"flag"
)

type ShowmeConfig struct {
	HostPort string
}

// call DefineFlags before myflags.Parse()
func (c *ShowmeConfig) DefineFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.HostPort, "hp", ":8080", "host:port (the host is optional)")
}

// call c.ValidateConfig() after myflags.Parse()
func (c *ShowmeConfig) ValidateConfig() error {
	return nil
}

package main

import (
	"flag"
)

type ShowmeConfig struct {
	Host string
	Port int
}

// call DefineFlags before myflags.Parse()
func (c *ShowmeConfig) DefineFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Host, "host", "", "host/ip to server on (optional)")
	fs.IntVar(&c.Port, "port", 8080, "port to serve index.html for images/R updates on.")
}

// call c.ValidateConfig() after myflags.Parse()
func (c *ShowmeConfig) ValidateConfig() error {
	return nil
}

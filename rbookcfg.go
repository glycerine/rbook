package main

import (
	"flag"
)

type RbookConfig struct {
	Host          string
	Port          int
	WsPort        int
	WssPort       int
	RbookFilePath string
}

// call DefineFlags before myflags.Parse()
func (c *RbookConfig) DefineFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Host, "host", "", "host/ip to server on (optional)")
	fs.IntVar(&c.Port, "port", 8080, "port to serve index.html for images/R updates on.")
	fs.StringVar(&c.RbookFilePath, "path", "", "path to the .rbook file to read and append to. this is also the default command line argument, so -path can be omitted in front of the path (default is my.rbook in the current dir)")
}

// call c.ValidateConfig() after myflags.Parse()
func (c *RbookConfig) ValidateConfig(fs *flag.FlagSet) error {

	if c.RbookFilePath == "" {
		args := fs.Args()
		if len(args) == 1 {
			c.RbookFilePath = args[0]
			//vv("set c.RbookFilePath to '%v'", c.RbookFilePath)
		} else {
			c.RbookFilePath = "my.rbook"
			//vv("defaulting c.RbookFilePath to '%v'", c.RbookFilePath)
		}
	}
	return nil
}

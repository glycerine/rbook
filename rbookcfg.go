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
	fs.IntVar(&c.Port, "port", 0, "port to serve index.html for images/R updates on (optional; picks anys free port by default but we'll try 8080 first if we can).")
	fs.StringVar(&c.RbookFilePath, "path", "", "path to the .rbook file to read and append to. this is also the default command line argument, so -path can be omitted in front of the path (default is my.rbook in the current dir)")
}

// call c.ValidateConfig() after myflags.Parse()
func (c *RbookConfig) FinishConfig(fs *flag.FlagSet) error {

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

	avail1, avail2, avail3 := GetAvailPort3()

	if c.WsPort == 0 {
		c.WsPort = avail1
	} else {
		if !IsAvailPort(c.WsPort) {
			AlwaysPrintf("c.WsPort %v not available, substituting port %v", c.WsPort, avail1)
			c.WsPort = avail1
		}
	}
	if c.WssPort == 0 {
		c.WssPort = avail2
	} else {
		if !IsAvailPort(c.WssPort) {
			AlwaysPrintf("c.WssPort %v not available, substituting port %v", c.WssPort, avail2)
			c.WssPort = avail2
		}
	}

	if c.Port == 0 {
		// try our dev default first, for simplicity.
		c.Port = 8080
	}

	if c.Port != 0 && !IsAvailPort(c.Port) {
		AlwaysPrintf("main web server c.Port %v not available, substituting port %v", c.Port, avail3)
		c.Port = avail3
	}

	if c.Port == 0 {
		c.Port = avail3
		AlwaysPrintf("main web server choosing port %v", c.Port)
	}

	return nil
}

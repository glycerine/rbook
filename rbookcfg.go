package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

type RbookConfig struct {
	Host string
	Port int

	WsHost  string // must fill something here to tell the client how to find us.
	WsPort  int
	WssPort int

	RbookFilePath string

	Rhome string

	// see .Process.Pid for PID
	xvfb   *exec.Cmd
	icewm  *exec.Cmd
	x11vnc *exec.Cmd

	Help bool

	Dump bool

	Wallpaper string
}

// call DefineFlags before myflags.Parse()
func (c *RbookConfig) DefineFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.Host, "host", "", "host/ip to server on (optional)")
	fs.IntVar(&c.Port, "port", 0, "port to serve index.html for images/R updates on (optional; if -port is taken or 0, defaults to the first free port at or above 8888)")
	fs.StringVar(&c.RbookFilePath, "path", "", "path to the .rbook file to read and append to. this is also the default command line argument, so -path can be omitted in front of the path (default is my.rbook in the current dir)")
	fs.StringVar(&c.Rhome, "rhome", "/usr/lib/R", "value of R_HOME to start R with. This directory should have contents: bin  COPYING  etc  lib  library  modules  site-library  SVN-REVISION")

	fs.BoolVar(&c.Help, "help", false, "show this help given rbook -h")
	fs.BoolVar(&c.Dump, "dump", false, "write script version of the -path binary book to standard out, then exit.")

	home := os.Getenv("HOME")
	fs.StringVar(&c.Wallpaper, "wall", fmt.Sprintf("%v/wallpaper", home), "path or symlink to wallpaper to set on the Xvfb/x11vnc")
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

	if c.Dump {
		if !FileExists(c.RbookFilePath) {
			return fmt.Errorf("rbook -dump could not find book to dump at path '%v'", c.RbookFilePath)
		}
		return nil // no web server stuff needed
	}

	// set the main web server port
	const maxPort int = 65535
	if c.Port == 0 || !IsAvailPort(c.Port) {
		// try our dev default first, for simplicity.
		c.Port = 8888
		if c.WsPort == 0 {
			c.WsPort = c.Port + 1
		}
		if c.WssPort == 0 {
			c.WssPort = c.Port + 2
		}

		// find the next incremental 3 ports above 8888, so we have
		// stability upon restarts, rather than a new random port each time.
		for !IsAvailPort(c.Port) || !IsAvailPort(c.WsPort) || !IsAvailPort(c.WssPort) {
			c.Port += 3
			c.WsPort = c.Port + 1
			c.WssPort = c.Port + 2
			if c.Port > maxPort-3 {
				panic("could not find available port for main rbook webserver")
			}
		}
		//c.Port = avail3 // ugh. works, but random port changes each time, requiring extra typing.
	}
	//AlwaysPrintf("main web server choosing port %v", c.Port)

	if c.Host == "" {
		// this means bind all interfaces, important to leave
		// it alone!
	}

	if c.WsHost == "" {
		if hostname != "" {
			c.WsHost = hostname
		} else {
			c.WsHost = GetExternalIP()
		}
	}

	avail1, avail2 := GetAvailPort2Excluding(c.Port)

	if c.WsPort == 0 {
		c.WsPort = avail1
	} else {
		if !IsAvailPort(c.WsPort) {
			//AlwaysPrintf("c.WsPort %v not available, substituting port %v", c.WsPort, avail1)
			c.WsPort = avail1
		}
	}
	if c.WssPort == 0 {
		c.WssPort = avail2
	} else {
		if !IsAvailPort(c.WssPort) {
			//AlwaysPrintf("c.WssPort %v not available, substituting port %v", c.WssPort, avail2)
			c.WssPort = avail2
		}
	}

	//vv("end of FinishConfig, c = '%#v'", c)
	return nil
}

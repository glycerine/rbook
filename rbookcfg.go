package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"4d63.com/tz"
)

const RFC3339NanoNumericTZ0pad = "2006-01-02T15:04:05.000000000-07:00"

const RFC3339MicroTz0 = "2006-01-02T15:04:05.000000Z07:00"

var UtcTz *time.Location
var NYC *time.Location
var Chicago *time.Location
var Frankfurt *time.Location
var London *time.Location
var IST *time.Location // Indian Standard Time
var Halifax *time.Location

func init() {
	initTimezonesEtc()
}

func initTimezonesEtc() {

	// do this is ~/.bashrc so we get the default.
	os.Setenv("TZ", "America/Chicago")

	var err error
	UtcTz, err = tz.LoadLocation("UTC")
	panicOn(err)
	NYC, err = tz.LoadLocation("America/New_York")
	panicOn(err)
	Chicago, err = tz.LoadLocation("America/Chicago")
	panicOn(err)
	Frankfurt, err = tz.LoadLocation("Europe/Berlin")
	panicOn(err)
	IST, err = tz.LoadLocation("Asia/Kolkata") // Indian Standard Time; UTC + 05:30
	panicOn(err)
	Halifax, err = tz.LoadLocation("America/Halifax")
	panicOn(err)
	London, err = tz.LoadLocation("Europe/London")
	panicOn(err)
}

type RbookConfig struct {
	Host string // leave empty to bind all interfaces
	Port int

	// Since Host empty means bind all interfaces, WsHost
	// is what we embed in the index.html as to where to
	// tell the client to websocket call back to.
	// Defaults to our hostname, then the first found
	// external IP address. Not currently a flag, but
	// could be.
	WsHost string // must fill something here to tell the client how to find us.

	WsPort  int
	WssPort int

	RbookFilePath string

	Rhome string

	// see .Process.Pid for PID
	xvfb   *exec.Cmd
	icewm  *exec.Cmd
	x11vnc *exec.Cmd

	Help bool

	Dump           bool
	DumpTimestamps bool

	Wallpaper string

	ShowVersion  bool
	ShowVersion2 bool

	Display string
}

// call DefineFlags before myflags.Parse()
func (c *RbookConfig) DefineFlags(fs *flag.FlagSet) {

	fs.BoolVar(&c.DumpTimestamps, "dumpts", false, "-dump but add timestamps to each line")
	fs.StringVar(&c.Host, "host", "", "host/ip to server on (optional)")
	fs.IntVar(&c.Port, "port", 0, "port to serve index.html for images/R updates on (optional; if -port is taken or 0, defaults to the first free port at or above 8888)")
	fs.StringVar(&c.RbookFilePath, "path", "", "path to the .rbook file to read and append to. this is also the default command line argument, so -path can be omitted in front of the path (default is my.rbook in the current dir)")
	fs.StringVar(&c.Rhome, "rhome", "/usr/lib/R", "value of R_HOME to start R with. This directory should have contents: bin  COPYING  etc  lib  library  modules  site-library  SVN-REVISION")

	fs.BoolVar(&c.Help, "help", false, "show this help given rbook -h")
	fs.BoolVar(&c.Dump, "dump", false, "write script version of the -path binary book to standard out, then exit.")

	home := os.Getenv("HOME")
	fs.StringVar(&c.Wallpaper, "wall", fmt.Sprintf("%v/.wallpaper", home), "path or symlink to wallpaper to set on the Xvfb/x11vnc")
	fs.BoolVar(&c.ShowVersion, "v", false, "show rbook version and exit")
	fs.BoolVar(&c.ShowVersion2, "version", false, "show rbook version and exit")

	fs.StringVar(&c.Display, "display", "", "X11 display number (example: -display :99) on which to display our X11 plots. Defaults to :10 but can be the string 'xvfb' (without quotes) if you want to start a new Xvfb based display to run on; however this can conflict with other Xvfb client programs (for unknown reasons) and so is not recommended.")
}

// call c.ValidateConfig() after myflags.Parse()
func (c *RbookConfig) FinishConfig(fs *flag.FlagSet) error {

	if c.ShowVersion || c.ShowVersion2 {
		fmt.Printf("%v\n", GetCodeVersion(ProgramName))
		os.Exit(0)
	}

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

	if c.Dump || c.DumpTimestamps {
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

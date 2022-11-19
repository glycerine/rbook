//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func systemCallSetGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(pid int) {
	// try to kill via PGID; we ran this child in its own process group for this.
	pgid, pgidErr := syscall.Getpgid(pid)
	if pgidErr == nil {
		syscall.Kill(-pgid, 9) // note the minus sign
	}
}

// Autmoate starting these support processes
// Xvfb :99 -screen 0 3000x2000x16 &
// icewm &
// feh --bg-scale ~/pexels-ian-turnell-709552.jpg
// x11vnc -display :99 -forever -nopw -quiet -xkb &
func (c *RbookConfig) StartXvfbAndFriends() {
	path := "/usr/bin/Xvfb"
	if !FileExists(path) {
		panic(fmt.Sprintf("could not find Xvfb at path '%v'", path))
	}

	disp := 99
	display := fmt.Sprintf(":%v", disp)
	os.Setenv("DISPLAY", display)
	args := strings.Split(display+" -screen 0 3000x2000x16", " ")
	c.xvfbCmd = exec.Command(path, args...)

	// put in its own process group so all sub-process are also
	// shut down and cleaned up if rbook is stopped.
	systemCallSetGroup(c.xvfbCmd)

	err := c.xvfbCmd.Start()
	panicOn(err)

}

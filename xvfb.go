//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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
//
// display example: ":99"
func (c *RbookConfig) StartXvfbAndFriends(display string) {

	// put in its own process group so all sub-process are also
	// shut down and cleaned up if rbook is stopped, using
	// systemCallSetGroup.

	c.xvfb = startInBackground("/usr/bin/Xvfb", strings.Split(display+" -screen 0 3000x2000x16", " ")...)
	c.icewm = startInBackground("/usr/bin/icewm")
	// give it a nice wallpaper
	go startInBackground("/usr/bin/feh", "--bg-scale", "misc/pexels-ian-turnell-709552.jpg").Wait()
	c.x11vnc = startInBackground("/usr/bin/x11vnc", "-display", display, "-forever", "-nopw", "-quiet", "-xkb")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	go func() {
		<-sigs
		c.StopXvfb()
	}()
}

func (c *RbookConfig) StopXvfb() {
	c.x11vnc.Process.Kill()
	c.icewm.Process.Kill()
	c.xvfb.Process.Kill()
}

func startInBackground(path string, args ...string) *exec.Cmd {
	if !FileExists(path) {
		panic(fmt.Sprintf("could not find path '%v'", path))
	}

	cmd := exec.Command(path, args...)

	// actually I think we specifically do not want this!
	//systemCallSetGroup(cmd)

	err := cmd.Start()
	panicOn(err)

	return cmd
}

//go:build linux || darwin

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	ps "github.com/mitchellh/go-ps"
)

func killProcessGroup(pid int) {
	// try to kill via PGID; useful if
	// we ran this child in its own process group.
	pgid, pgidErr := syscall.Getpgid(pid)
	if pgidErr == nil {
		syscall.Kill(-pgid, 9) // note the minus sign
	}
}

// only used to have each process start its own
// new process group, which is the opposite of what
// we want here.
func systemCallSetGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Autmoate starting these support processes
// Xvfb :99 -screen 0 3000x2000x16 &
// icewm &
// feh --bg-scale ~/pexels-ian-turnell-709552.jpg
// x11vnc -display :99 -forever -nopw -quiet -xkb &
//
// display example: ":99"
func (c *RbookConfig) StartXvfbAndFriends(display string) {

	// by default these are all in the process group
	// of rbook, which is desired for clean shutdown.

	c.xvfb = startInBackground("/usr/bin/Xvfb", strings.Split(display+" -screen 0 3000x2000x16", " ")...)
	c.icewm = startInBackground("/usr/bin/icewm")
	// give it a nice wallpaper
	go startInBackground("/usr/bin/feh", "--bg-scale", "misc/pexels-ian-turnell-709552.jpg").Wait()
	c.x11vnc = startInBackground("/usr/bin/x11vnc", "-display", display, "-forever", "-nopw", "-quiet", "-xkb")

	// sigs := make(chan os.Signal, 1)
	// signal.Notify(sigs, syscall.SIGTERM)
	// go func() {
	// 	<-sigs
	// 	c.StopXvfb()
	// }()
}

func (c *RbookConfig) StopXvfb() {

	// not good: leaving orphaned shm segments
	//c.x11vnc.Process.Kill()
	//c.icewm.Process.Kill()
	//c.xvfb.Process.Kill()

	// try giving chance to clean up. Yes, much better.
	// No more orphaned seen in ipcs -m
	c.x11vnc.Process.Signal(syscall.SIGTERM)
	c.icewm.Process.Signal(syscall.SIGTERM)
	c.xvfb.Process.Signal(syscall.SIGTERM)

	//vv("killed x11vnc, icewm, Xvfb")
}

func startInBackground(path string, args ...string) *exec.Cmd {
	if !FileExists(path) {
		panic(fmt.Sprintf("could not find path '%v'", path))
	}

	cmd := exec.Command(path, args...)

	err := cmd.Start()
	panicOn(err)

	return cmd
}

// read the command lines of all Xvfb processes from /proc
// and pick an unused DISPLAY number.
func GetAvailXvfbDisplay() int {

	r := make(map[int]bool)

	ps, err := ps.Processes()
	if err != nil {
		panic(err)
	}
	for _, proc := range ps {
		if proc.Executable() == "Xvfb" {
			cmdline := fmt.Sprintf("/proc/%d/cmdline", proc.Pid())
			if FileExists(cmdline) {
				by, err := ioutil.ReadFile(cmdline)
				panicOn(err)
				// C-style array of strings, 0 terminated.
				split := bytes.Split(by, []byte{0})
				for j := range split {
					arg := string(split[j])
					//vv("have arg '%v' from pid %v", arg, proc.Pid())
					if len(arg) >= 2 && arg[0] == ':' {
						n, err := strconv.Atoi(arg[1:])
						if err == nil {
							r[n] = true
						}
						//vv("added n = %v", n)
					}
				}
			}
		}
	}
	// try 30 - 199
	for i := 30; i < 200; i++ {
		if !r[i] {
			return i
		}
	}
	panic("could not get available Xvfb DISPLAY, tried 30 - 199")
}

// NB: x11vnc defunct diagnosis: during dev we probably left
//     alot of orphaned shared memory, leading to running out.
//
// in a new shell, set DISPLAY and run x11vnc. Saw:
//
// 19/11/2022 16:15:06 shmget(scanline) failed.
// 19/11/2022 16:15:06 shmget: No space left on device
//
// So x11vnc could not get a shm seg, clear out all the old ones with:
//
// for i in `ipcs -m|awk '$6==0 {print $2}'`; do ipcrm shm $i; done
//
// See also https://serverfault.com/questions/371068/shared-memory-shmget-fails-no-space-left-on-device-how-to-increase-limits
//   which says:
// "Use ipcs -l to check the limits actually in force, and
//  ipcs -a and ipcs -m to see what is in use, so you can
//  compare the output. Look at the nattch column: are there
//  segments with no processes attached that were not removed
//  when processes exited (which normally means the program crashed)?
//  ipcrm can clear them, although if this is a test machine, a
//  reboot is quicker (and will make sure your changes to
//  limits are picked up)."
//
// Update: we use SIGTERM now to shutdown, and x11vnc/icewm/Xvfb now
//         clean up after themselves better.

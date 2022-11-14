// home: https://github.com/glyerine/vprint
// Copyright(C) 2019 Jason E. Aten, Ph.D. All rights reserved.
// License: MIT

package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"
	"time"
)

const RFC3339MsecTz0 = "2006-01-02T15:04:05.000Z07:00"

// for tons of debug output
var VerboseVerbose bool = false

// convience functions for . import
var pp = PP
var vv = VV

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

func PP(format string, a ...interface{}) {
	if VerboseVerbose {
		TSPrintf(format, a...)
	}
}

func VV(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

func AlwaysPrintf(format string, a ...interface{}) {
	TSPrintf(format, a...)
}

var tsPrintfMut sync.Mutex

// time-stamped printf
func TSPrintf(format string, a ...interface{}) {
	tsPrintfMut.Lock()
	Printf("\n%s %s ", FileLine(3), ts())
	Printf(format+"\n", a...)
	tsPrintfMut.Unlock()
}

// get timestamp for logging purposes
func ts() string {
	return time.Now().Format(RFC3339MsecTz0)
}

// so we can multi write easily, use our own printf
var OurStdout io.Writer = os.Stdout

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.
func Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(OurStdout, format, a...)
}

func FileLine(depth int) string {
	_, fileName, fileLine, ok := runtime.Caller(depth)
	var s string
	if ok {
		s = fmt.Sprintf("%s:%d", path.Base(fileName), fileLine)
	} else {
		s = ""
	}
	return s
}

func Caller(upStack int) string {
	// elide ourself and runtime.Callers
	target := upStack + 2

	pc := make([]uintptr, target+2)
	n := runtime.Callers(0, pc)

	f := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(pc[:n])
		for i := 0; i <= target; i++ {
			contender, more := frames.Next()
			if i == target {
				f = contender
			}
			if !more {
				break
			}
		}
	}
	return f.Function
}

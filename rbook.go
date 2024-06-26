package main

// Copyright (C) 2023 Jason E. Aten, Ph.D. All rights reserved.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/glycerine/blake2b-simd"
	"github.com/glycerine/cryrand"
	"github.com/glycerine/embedr"
)

var _ = syscall.Getpgid

func init() {
	// this important protection. R will crash if it gets SIGINT under --no-readline,
	// which we always use now because readline re-writes signal handler for
	// SIGINT without the SA_ONSTACK flag, which panics the go runtime when seen.
	// So we must intercept SIGINT now.
	//
	// Hmm... but then it looks like even without readline, R installs
	// its own SIGINT handler. Which is nice in that it lets us ctrl-c
	// out of partial input. Our ReplDLLinit() go wrapper does
	// C.set_SA_ONSTACK() too, so hopefully even without this (which
	// does seem to get displayed quickly by R, we'll not crash.
	// Leaving it here as extra cover, in case a SIGINT comes in
	// before R's is set.
	intercept_SIGINT()

	// Arrange that main.main runs on main thread. This lets R startup
	// without crashing when run on a non-main thread.
	runtime.LockOSThread()
	var err error
	hasher, err = blake2b.New(nil)
	panicOn(err)
	hostname, err = os.Hostname()
	panicOn(err)
	username = os.Getenv("USER")

	sep = string(os.PathSeparator)

	/*	if m, err := TerminalMode(); err == nil {
			origTerminalMode = m.(*termios)
			vv("set origTerminalMode = '%#v'", origTerminalMode)
		}
	*/
}

var username string
var hostname string
var hasher hash.Hash
var sep string

// PathHash gets attached to all image requests
// as a ?pathhash=0248... query parameter. It includes the
// hostname and path to the file on the host. This is
// passed to the browser so it can request the most
// recent file; so we don't view stale cached graphics
// by mistake when the browser sees the same .png file
// name. And if the file actually is the same, from
// the same host and the same path, well then let
// the browser skip fetching, since the content and
// origin must be identical and the browser cache
// is working as designed.
func PathHash(path string) (hash string, imageBy []byte) {
	hasher.Reset()
	hasher.Write([]byte(hostname + ":" + path + ":"))
	by, err := ioutil.ReadFile(path)
	panicOn(err)
	hasher.Write(by)
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil)), by
}

// provide a way for rbook -viewonly to turn off ctrl-c intercept.
var stopMonitoringSIGINT func()

func intercept_SIGINT() {
	//vv("intercept_SIGINT installing")
	c := make(chan os.Signal, 100)
	stopMonitoringSIGINT = func() {
		signal.Stop(c) // signal package will never send on c now
		close(c)       // the go routine below will exit.
	}
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, ctrl-c, handle it
			_ = sig
			fmt.Printf("rbook got SIGINT... setting R_interrupts_pending = 1.\n")
			embedr.SetR_interrupts_pending()
		}
	}()
}

func ignore_SIGPIPE() {

	// Alternative approach: just call signal.Ignore(syscall.SIGPIPE).
	// ref: https://github.com/golang/go/issues/32386
	signal.Ignore(syscall.SIGPIPE)

	// calling signal.Notify for SIGPIPE means that if we get a
	// broken pipe on stdin or stdout, then
	// the golang runtime will not exit with 141 error code to the shell.
	// https://pkg.go.dev/os/signal#hdr-SIGPIPE
	/*
		//vv("intercept_SIGPIPE installing")
		c := make(chan os.Signal, 100)
		signal.Notify(c, syscall.SIGPIPE)
		go func() {
			for sig := range c {
				_ = sig
				fmt.Printf("rbook got SIGPIPE... ignoring\n")
			}
		}()
	*/
}

var globalUDLock *UDLock

// avoid leaving dangling Xvfb/x11vnc/icewm when we kill the *R*
// session from within emacs.
func (cfg *RbookConfig) intercept_SIGTERM_and_cleanup() {
	ch := make(chan os.Signal, 100)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		<-ch
		if globalUDLock != nil {
			globalUDLock.Close()
		}
		cfg.StopXvfb()
		//fmt.Printf("rbook got SIGTERM and stopped helpers.")
		// stop listening for SIGTERM, then send it again.
		signal.Stop(ch)
		signal.Reset(syscall.SIGTERM)
		// let R shutdown
		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
}

func main() {

	// crashes orphan shm segments, clean them up.
	CleanupOrphanedSharedMemorySegmentsFromXvfbCrashes()

	// try to not exit to bash with 141 (SIGPIPE error, signal 13) on occassion.
	ignore_SIGPIPE()

	cfg := &RbookConfig{}
	// there will be R arguments we don't recognize, so
	// ContinueOnError
	myflags := flag.NewFlagSet("rbook", flag.ContinueOnError)

	// suppress the flag errors when ess/emacs passes -no-readline or other R flags
	var flagerr bytes.Buffer
	myflags.SetOutput(&flagerr)

	cfg.DefineFlags(myflags)

	err := myflags.Parse(os.Args[1:])
	if err == flag.ErrHelp {
		fmt.Printf("%v\n", flagerr.String())
		os.Exit(1)
	}
	if err != nil {
		errs := err.Error()
		if strings.HasPrefix(errs, "flag provided but not defined:") {
			// just ignore -no-readline and any other R flags
		} else {
			vv("err on myflags.Parse(): '%v'", err.Error())
			vv("flagerr = '%v'", flagerr.String())
			panic("fixme?")
		}
	}

	err = cfg.FinishConfig(myflags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s command line flag error: '%s'", ProgramName, err)
		os.Exit(1)
	}

	if cfg.ViewOnly {
		// skipping R session, just showing .png; so
		// make the minimum possible book that showme needs
		skeleton := &HashRBook{
			path2image: make(map[string]*HashRElem),
		}
		stopMonitoringSIGINT() // allow ctrl-c to shutdown.
		StartShowme(cfg, skeleton)
		select {} // hang forever
	}

	fn := cfg.RbookFilePath
	cwd, err := os.Getwd()
	panicOn(err)
	bookpath := cwd + sep + fn
	if len(fn) > 0 && (filepath.IsAbs(fn) || fn[0] == '/' || fn[0] == '~') {
		// we have an absolute path, prepending cwd will make a mess.
		bookpath = fn
	}

	if false { // runtime.GOOS == "darwin" {
		// unix domain sockets buggy on darwin/go1.21.0 ?
		// https://github.com/golang/go/issues/62337
	} else if cfg.Dump || cfg.DumpTimestamps {
		// don't bother locking if we are just going to -dump or -dumpts and exit.
	} else {
		// lock boopath, or find out somebody else already has it locked.
		udlock, err := NewUDLock(bookpath)
		if err != nil {
			panic(fmt.Sprintf("'%v' is already in use: '%v'", bookpath, err))
		}
		globalUDLock = udlock
		defer udlock.Close()
	}

	// Generate an R script too.
	// The script will have text version of the binary .rbook, written
	// in parallel, for ease reference. Obviously it will be missing
	// the plots; but we could write their paths in.
	//
	scriptPath := bookpath + ".rsh"
	var script *os.File
	freshScript := false
	if !FileExists(scriptPath) {
		freshScript = true
	}
	if cfg.Dump || cfg.DumpTimestamps {
		// we are dumping the binary to stdout in script/text format.
		script = os.Stdout
		freshScript = true
	} else {
		script, err = os.OpenFile(scriptPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0770)
		panicOn(err)
	}
	var history *HashRBook

	history, appendFD, err := ReadBook(username, hostname, bookpath)
	panicOn(err)
	if false {
		// don't need to hold mut b/c reload server not started yet
		vv("see history len %v:", len(history.elems))
		for _, e := range history.elems {
			fmt.Printf("%v\n", e)
		}
	}
	_ = appendFD
	_ = history

	scriptID := fmt.Sprintf(`
#%v@%v:%v
#BookID:%v
#R rbook created: %v
`, username, hostname, bookpath, history.BookID, history.CreateTm.Format(RFC3339NanoNumericTZ0pad))

	// write header for the script
	if freshScript {
		fmt.Fprintf(script, `#!/bin/bash
exec R --vanilla -q --slave -e "source(file=pipe(\"tail -n +3 $0\"))" --args $@

# text version of:

%v

require(png)

`, scriptID)
	}

	if cfg.Dump || cfg.DumpTimestamps {
		cfg.dumpToScript(script, history)
		os.Exit(0)
	}

	// for each new websocket client, as they
	// arrive, replay the history for them.

	// As we enter new commands, create new console output,
	// and generate images, save each to the history struct
	// and to the file.

	// start R first so it gets the main thread.

	// We start up Xvfb on a free DISPLAY like :30, :31, :32...,
	// and put these child processes each in their own distinct process
	// group. In isolated groups, they are then unaffected
	// if a ctrl-c should get through, as was happening before we
	// banned readline by always starting with the --no-readline flag.
	// Also we figured out how to get R to callback on q() quit,
	// by using .Last.sys below, so we can terminate these helpers
	// reliably now with our StopXvfb().
	//
	// Xvfb :99 -screen 0 3000x2000x16 &
	// icewm &
	// feh --bg-scale ~/pexels-ian-turnell-709552.jpg
	// x11vnc -display :99 -forever -nopw -quiet -xkb &

	os.Setenv("R_HOME", cfg.Rhome)
	fmt.Printf("we set: export R_HOME=%v\n", cfg.Rhome)

	fmt.Printf("rbook version: %v\n", GetCodeVersion(ProgramName))

	fmt.Printf("cwd: %v\n\n%v\n", cwd, scriptID)

	if cfg.Display == "xvfb" {
		disp := GetAvailXvfbDisplay()
		display := fmt.Sprintf(":%v", disp)
		os.Setenv("DISPLAY", display)
		vncPort := cfg.StartXvfbAndFriends(display)
		fmt.Printf("Xvfb using DISPLAY=:%v  R_HOME=%v  vncPort=%v\n", disp, cfg.Rhome, vncPort)

	} else if cfg.Display == "png" {
		fmt.Printf("Writing to png files directly for plots.\n")
	} else {
		if cfg.Display == "" {
			cfg.Display = ":10"
			fmt.Printf("rbook using (default) DISPLAY=:10\n")
		} else {
			if cfg.Display[0] != ':' {
				panic(fmt.Sprintf("rbook -display argument '%v' did not start with ':'", cfg.Display))
			}
			mustBeNumber := string([]byte(cfg.Display)[1:])
			num, err := strconv.Atoi(mustBeNumber)
			if err != nil {
				panic(fmt.Sprintf("rbook -display argument '%v' did not have a number following the ':'", cfg.Display))
			}
			if num < 0 {
				panic(fmt.Sprintf("rbook -display argument '%v' was a negative number", cfg.Display))
			}
			fmt.Printf("rbook using -display specified DISPLAY=%v\n", cfg.Display)
		}
		os.Setenv("DISPLAY", cfg.Display)
	}

	// initialize the embedded R.
	embedr.InitR(true)
	defer embedr.EndR()

	cfg.intercept_SIGTERM_and_cleanup()

	// ESS hates this. It makes ctrl-a to move to
	// beginning of line mess up and move before the > prompt.
	// So leave it out for now until we (maybe) can patch ESS.
	//
	// updatePromptCwd := func(prefix string) {
	// 	// prefix could be used for git branch
	// 	cwd, err := os.Getwd()
	// 	panicOn(err)
	// 	// keep just the last 2 dir
	// 	splt := strings.Split(cwd, sep)
	// 	n := len(splt) - 2
	// 	if n < 0 {
	// 		n = 0
	// 	}
	// 	embedr.SetCustomPrompt(prefix + strings.Join(splt[n:], sep) + "_> ")
	// }
	//updatePromptCwd("")

	// don't need to hold mut mutex here b/c reload server not started yet
	seqno := len(history.elems)

	lastCommandLineNum := getLastCommandLineNum(history)

	StartShowme(cfg, history) // serve the initial html and the png files to the web browsers
	//vv("Showme http server started. Starting reload websocket server.")
	cfg.startReloadServer(history) // websockets to tell browsers what to show when there's an update.
	//vv("Reload server started.")

	// number the saved png files.
	nextSave := 0

	archiveElem := func(e *HashRElem) {
		// CODEX: keep in sync with the svvPlot() and dvvFunc() CODEX above.
		history.mut.Lock()
		history.elems = append(history.elems, e)
		if e.ImagePath != "" {
			//vv("saving e.ImagePath '%v' to path2image", e.ImagePath)
			history.path2image[e.ImagePath] = e
		}
		history.mut.Unlock()

		by, err := e.SaveToSlice()
		panicOn(err)

		// try to detect if file needs to be re-opened to continue to append:
		var preSize, postSize int64
		preSize, err = FileSize(bookpath)
		// allow error here because if file was deleted we will get a "no such file" error.

		if err != nil || preSize == 0 {
			vvlog("somebody has deleted our book: '%v'. re-creating it from memory.", bookpath)
			appendFD.Close() // try not the leak the old fd.
			appendFD = history.DeletePathAndReSaveFullBook(bookpath)
			// the latest e is already written so we are done now.
			return
		}

		_, err = appendFD.Write(by)
		panicOn(err)
		// flush to disk
		err = appendFD.Sync()
		panicOn(err)

		postSize, err = FileSize(bookpath)
		panicOn(err)

		if len(by) > 0 {
			// we expect postSize to be == preSize + len(by) now.
			if postSize <= preSize {
				// probably git deleted and replaced our file. Re-open.
				vvlog("detected write not hitting disk: '%v' pre-write size: %v; post-write size: %v; will re-open file.", bookpath, preSize, postSize)

				// assume that history will be the same, like the scenario that
				// prompted this addition: git/rebase deleted and re-created our file.
				var history2 *HashRBook
				var appendFD2 *os.File
				history2, appendFD2, err = ReadBook(username, hostname, bookpath)
				panicOn(err)
				appendFD = appendFD2

				history.mut.Lock()
				nelem := len(history.elems)
				history.mut.Unlock()

				if len(history2.elems) != nelem-1 || history2.BookID != history.BookID {
					// we should just be missing the one that did not hit disk.
					vvlog("unknown serialization problem: len(history2.elems)=%v; but nelem is not 1 more; (or BookID changed) nelem=len(history.elems) = %v; bookpath='%v'; attempting to re-write full rbook to disk from memory, if we can.", len(history2.elems), nelem, bookpath)
					appendFD.Close() // try not the leak the old fd.
					appendFD = history.DeletePathAndReSaveFullBook(bookpath)
					// the latest e is already written so we are done now.
					return
				}

				// try to append again, after the re-open

				_, err = appendFD.Write(by)
				panicOn(err)

				// flush to disk
				err = appendFD.Sync()
				panicOn(err)

				postSize, err = FileSize(bookpath)
				panicOn(err)

				if postSize <= preSize {
					vvlog("detected write STILL not hitting disk after re-open: '%v' pre-write size: %v; post-write size: %v; about to panic.", bookpath, preSize, postSize)
					panic(fmt.Sprintf("could not append to binary book file: '%v': pre-write size: %v; post-write size: %v", bookpath, preSize, postSize))
				}

				// also need to fix the my.rbook.hostname.rsh file. /proc/pid/fd will show as deleted.
				if script != nil {
					script.Close()
				}
				script, err = os.OpenFile(scriptPath, os.O_CREATE|os.O_RDWR|os.O_APPEND|os.O_TRUNC, 0770)
				panicOn(err)
				cfg.dumpToScript(script, history)
				err = script.Sync()
				panicOn(err)
			}
		}
	}

	// setup for svvPlot() to be able to use -display=png and not need X11/cairo stuff.
	odirPlots := bookpath + ".plots"
	panicOn(os.MkdirAll(odirPlots, 0777))
	var nextPlotSavePath string
	if cfg.Display == "png" {
		// start a png() file, reading to capture any plot.
		rnd20 := cryrand.RandomStringWithUp(20)
		nextPlotSavePath = fmt.Sprintf("%v/plotmini_%03d_%v.png", odirPlots, nextSave, rnd20)
		err = embedr.EvalR(fmt.Sprintf(`png(filename='%v', height=700, width=700, bg="white", type="cairo-png")`, nextPlotSavePath))
		if err != nil {
			vv("error during initial png(filename='%v'): '%v'", nextPlotSavePath, err)
			panic("could not start png file saving!")
		}
	}

	svvPlot := func() {
		//fmt.Printf("svvPlot() called!  seqno=%v, bookpath='%v'\n", seqno, bookpath)

		e := &HashRElem{
			Tm:    time.Now(),
			Seqno: seqno,
		}

		if cfg.Display == "png" {
			// save the .png file
			err = embedr.EvalR(fmt.Sprintf(`dev.off()`))
			panicOn(err)
		} else {
			rnd20 := cryrand.RandomStringWithUp(20)
			nextPlotSavePath = fmt.Sprintf("%v/plotmini_%03d_%v.png", odirPlots, nextSave, rnd20)
			if runtime.GOOS == "darwin" {
				err = embedr.EvalR(fmt.Sprintf(`quartz.save(file='%v', type = "png", device = dev.cur(), dpi = 100, bg="white")`, nextPlotSavePath))
			} else {
				err = embedr.EvalR(fmt.Sprintf(`savePlot(filename="%v")`, nextPlotSavePath))
			}
		}
		if err != nil {
			// possibly "no plot on device to save";
			// don't bother to send to browser. And don't crash.
			//continue
			vv("error during savePlot(filename='%v'): '%v'", nextPlotSavePath, err)
			return
		}
		panicOn(err)
		pathhash, imageby := PathHash(nextPlotSavePath)
		//vv("saved to path = '%v'; pathhash='%v'", nextPlotSavePath, pathhash)
		nextSave++

		//vv("Reloading browser with image path '%v'", nextPlotSavePath)
		msg := prepImageMessage(nextPlotSavePath, pathhash, seqno)

		e.Typ = Image
		e.ImageJSON = msg
		e.ImageHost = hostname
		e.ImagePath = nextPlotSavePath
		e.ImageBy = imageby
		e.ImagePathHash = pathhash
		e.msg = []byte(msg)

		script = writeScriptImage(script, nextPlotSavePath)

		hub.broadcast <- e
		seqno++

		archiveElem(e)

		if cfg.Display == "png" {
			// After nextPlotSavePath is done being referenced; and after
			// nextSave has been incremented:
			// start the next png(), reading to capture any plot.
			rnd20 := cryrand.RandomStringWithUp(20)
			nextPlotSavePath = fmt.Sprintf("%v/plotmini_%03d_%v.png", odirPlots, nextSave, rnd20)
			// tried rebuilding R to not need x11 but to keep png, for headless vps:
			// ./configure --with-x=no --with-cairo=yes --with-libpng=yes --enable-R-shlib
			//
			err = embedr.EvalR(fmt.Sprintf(`png(filename='%v', height=700, width=700, bg="white", type="cairo-png")`, nextPlotSavePath))
			if err != nil {
				panic(fmt.Sprintf("error during subsequent png(filename='%v'): '%v'", nextPlotSavePath, err))
			}
		}

		/*
			// CODEX: keep in sync with code after the switch below!
			history.mut.Lock()
			history.elems = append(history.elems, e)
			if e.ImagePath != "" {
				//vv("saving e.ImagePath '%v' to path2image", e.ImagePath)
				history.path2image[e.ImagePath] = e
			}
			history.mut.Unlock()

			by, err := e.SaveToSlice()
			panicOn(err)
			_, err = appendFD.Write(by)
			panicOn(err)
		*/
	}

	setWebDataFunc := func() {
		dat, err := embedr.EvalR_fullback(`.my.webData`)
		panicOn(err)
		// strings come through as []string
		// data frames come through as map[string]interface{} where
		// the interface{} is []float64{} with the columns.
		//vv("setWebDataFunc called. dat = '%#v'", dat)
		//		gJsonCandles = make([]byte, len(dat))
		//		copy(gJsonCandles, dat)

		switch x := dat.(type) {
		default:
			vv("don't know what to do with dat type '%T'", dat)
		case map[string]interface{}:
			// turn it into gJsonCandles

			var tms []string
			var f [][]float64
			var fnames []string
			name2col := make(map[string]int)
			for fld, col := range x {
				switch y := col.(type) {
				case []string:
					if fld == "tm" {
						tms = y
					} else {
						vv("not sure what to do with a string dataframe column that is not 'tm' but instead named: '%v'", fld)
						panic(fmt.Sprintf("not sure what to do with a string dataframe column that is not 'tm' but instead named: '%v'", fld))
					}
				case []float64:
					fnames = append(fnames, fld)
					name2col[fld] = len(f)
					f = append(f, y)
				default:
					vv("unsure what to do with column '%v' of type %T", fld, col)
					panic(fmt.Sprintf("unsure what to do with column '%v' of type %T", fld, col))
				}
			}

			buf := &bytes.Buffer{}
			// [["2004-01-02",10452.74,10409.85,10367.41,10554.96,168890000], ... ]
			fmt.Fprintf(buf, "[")

			//vv("starting on i loop")
			for i, tm := range tms {
				if i > 0 {
					fmt.Fprintf(buf, ",")
				} else {
					//vv("debug, buf = '%v'", string(buf.Bytes()))
				}
				// tm is already a string, no need
				// for .In(Chicago).Format(RFC3339MicroNumericTZ) stuff.
				fmt.Fprintf(buf, `["%v"`, tm)

				// go in strict Open,Close,Low,High,Volume order, which echart expects.
				for _, nm := range []string{"op", "cl", "lo", "hi", "sz"} {
					j, ok := name2col[nm]
					if !ok {
						vv("could not locate expected column '%v' in '%#v'; must be labelled: tm,op,cl,lo,hi,sz", nm, name2col)
						panic(fmt.Sprintf("could not locate expected column '%v' in '%#v'; must be labelled: tm,op,cl,lo,hi,sz", nm, name2col))
					}
					//vv("trying to access j=%v from len f = %v", j, len(f))
					fmt.Fprintf(buf, ",%v", f[j][i]) // print value from row i, column j.

					//vv("ok at i = %v", i)
				}
				fmt.Fprintf(buf, "]")
			}
			fmt.Fprintf(buf, "]")

			//vv("done just fine with gJsonCandles translation")
			gJsonCandles = buf.Bytes()
		}
	}

	dvvFunc := func() {

		// collect any text in the sink
		sinkgot, err := embedr.EvalR_fullback(`zrecord_mini_console`)
		panicOn(err)
		capture, capturedOutputOK := sinkgot.([]string)

		fmt.Printf("dvvFunc() called!  seqno=%v, capture='%v'; capturedOutputOK=%v\n", seqno, capture, capturedOutputOK)

		captureJSON := ""
		if capturedOutputOK {
			var newlines string

			//vv("capture = %v lines\n", len(capture))
			for _, line := range capture {
				//fmt.Printf("line %02d: %v\n", i, line)
				newlines += line + "\n"
				esc, _ := escape(line)
				if captureJSON == "" {
					captureJSON += fmt.Sprintf(`"## %v"`, esc)
				} else {
					captureJSON += fmt.Sprintf(`,"## %v"`, esc)
				}
			}
			//vv("captureJSON='%v'", captureJSON)
			captureJSON = `[` + captureJSON + `]`
		}

		if !capturedOutputOK || captureJSON == "" {
			return
		}
		// reset the sink, so if we call dvv() again in a loop, we won't repeat ourselves.
		embedr.EvalR(`sink(file=NULL)`)
		embedr.EvalR(`if(exists("zrecord_mini_console")) { rm("zrecord_mini_console") }`)
		embedr.EvalR(`sink(textConnection("zrecord_mini_console", open="w"), split=T);`)

		e := &HashRElem{
			Tm:    time.Now(),
			Seqno: seqno,
		}

		//vv("prepConsoleMessages(captureJSON='%v', seqno='%v')", captureJSON, seqno)
		msg := prepConsoleMessage(captureJSON, seqno)
		e.Typ = Console
		e.ConsoleJSON = msg
		e.msg = []byte(msg)

		// append to our text file version on disk
		script = writeScriptConsole(script, capture)

		hub.broadcast <- e
		seqno++

		archiveElem(e)
		/*
			// CODEX: keep in sync with code after the switch below!
			history.mut.Lock()
			history.elems = append(history.elems, e)
			if e.ImagePath != "" {
				//vv("saving e.ImagePath '%v' to path2image", e.ImagePath)
				history.path2image[e.ImagePath] = e
			}
			history.mut.Unlock()

			by, err := e.SaveToSlice()
			panicOn(err)
			_, err = appendFD.Write(by)
			panicOn(err)
		*/
	}

	// our repl
	embedr.ReplDLLinit()
	embedr.SetGoCallbackForCleanup(func() { cfg.StopXvfb() })
	embedr.SetRCallbackToGoFunc(svvPlot)
	embedr.SetRCallbackToGoFuncDvv(dvvFunc)
	embedr.SetRCallbackToGoFuncSetWebData(setWebDataFunc)

	// In .Last.sys,
	// do graphics.off() first to try and avoid q() resulting in:
	//
	//Error in .Internal(quit(save, status, runLast)) :
	//  X11 fatal IO error: please save work and shut down R
	//>
	embedr.EvalR(`.Last.sys=function(){graphics.off();.C("CallGoCleanupFunc")}`)

	// cannot do invisible(TRUE) inside here; as that will
	// hide the previous command output!
	embedr.EvalR(`sv=function(...){}`) // easy to type. cmd == "sv()" : save the current graph (to browser).
	embedr.EvalR(`dv=function(...){}`) // easy to type. cmd == "dv()" : display the last printed output (in browser).

	// for in an R program loop... save the current graph (to browser).
	embedr.EvalR(`svv=function(...){ .C("CallRCallbackToGoFunc"); c()}`)
	embedr.EvalR(`dvv=function(...){ .C("CallRCallbackToGoFuncDvv"); c()}`)
	embedr.EvalR(`.my.webData <<- c();`)
	embedr.EvalR(`setweb=function(webData){ .my.webData <<- webData; .C("CallRCallbackToGoFuncSetWebData"); c()}`)

	// on darwin, we need to start a quartz window with
	// the bg="white", or else the browser will get an opaque
	// background which can look invisible (dark gray on black).
	// So don't let quartz() happen implicitly--deliberately start
	// a quartz window with a white background first; and just
	// try to reuse this plot. If starting a new plot, well need this;
	// so maybe alias x11 = function() { quartz(bg="white"); } on darwin
	// as a convenience and make things the same as on linux.
	if runtime.GOOS == "darwin" {
		embedr.EvalR(`quartz(bg="white")`)
		embedr.EvalR(`x11=function() {quartz(bg="white")}`)
		// see also
		// https://doingbayesiandataanalysis.blogspot.com/2015/05/graphics-window-for-macos-and-rstudio.html
		// for hints on doing cross-platform plots; e.g. under RStudio, Windoze, etc.
	}

	// need to save one console capture back for dv() recording of output, since dv() itself will be a command.
	captureJSON := ""
	prevJSON2 := ""
	prevJSON := ""
	var captureOK []string
	var prevCaptureOK []string
	var captureHistoryJSON []string
	var captureHistory []string
	_ = captureHistoryJSON
	_ = captureHistory
	// (list "" '(("..." . "")) '("..."))
	// (list "" '((" " . "")) '(""))

	// more garbage that slipped through the above check:
	// (list "stats" '(("x" . "") ("df1" . "") ("df2" . "") ("ncp" . "") ("log" . "FALSE")) '("x" "df1" "df2" "ncp" "log"))
	var capture []string
	var capturedOutputOK bool
	var lastHistory string

	for {

		//updatePromptCwd("")
		embedr.EvalR(`if(exists("zrecord_mini_console")) { rm("zrecord_mini_console") }`)
		embedr.EvalR(`sink(textConnection("zrecord_mini_console", open="w"), split=T);`)

		//path := ""
		did := embedr.ReplDLLdo1()
		_ = did
		//vv("did = %v", did)
		if did > 1 {
			// did == 2: this seems to mean that the parse is incomplete; need more input.
			//vv("back from one call to R_ReplDLLdo1(); did = %v\n", did)
		}
		// did == 0 => error evaluating
		// did == -1 => ctrl-d (end of file).

		lastHistory = embedr.LastHistoryLine() // to check for trailing semicolon
		trailingSemicolon := strings.HasSuffix(lastHistory, ";")
		autoDV := !trailingSemicolon
		_ = autoDV
		//vv("lastHistory = '%v'; trailingSemicolon = %v", lastHistory, trailingSemicolon)

		sinkgot, err := embedr.EvalR_fullback(`zrecord_mini_console`)

		//panicOn(err) // RevalErr, can happen from ctrl-c terminate. let us not crash:
		if err != nil {
			vv("error requesting zrecord_mini_console: '%v'", err)
			continue
		}
		capture, capturedOutputOK = sinkgot.([]string)

		if capturedOutputOK {
			prevCaptureOK = captureOK
			captureOK = capture

			prevJSON2 = prevJSON
			prevJSON = captureJSON
			captureJSON = ""
			var newlines string

			//vv("capture = %v lines\n", len(capture))
			for _, line := range capture {
				//fmt.Printf("line %02d: %v\n", i, line)
				if isGarbage(line) {
					continue
				}
				newlines += line + "\n"
				esc, grew := escape(line)
				_ = grew
				//if grew > 0 {
				//	vv("see grew = %v on line '%v'", line)
				//	vv("esc version = '%v'", esc)
				//}
				if captureJSON == "" {
					captureJSON += fmt.Sprintf(`"## %v"`, esc)
				} else {
					captureJSON += fmt.Sprintf(`,"## %v"`, esc)
				}
			}
			captureHistoryJSON = append(captureHistoryJSON, captureJSON)
			captureJSON = `[` + captureJSON + `]`
			captureHistory = append(captureHistory, newlines)
		}
		//vv("prevJSON = '%v'", prevJSON)
		//vv("prevJSON2 = '%v'", prevJSON2)
		//vv("captureJSON = '%v'", captureJSON)

		// Fortunately this does not appear to disturb Lastexpr().
		// Likewise, errors do not make it to Lastexpr() on purpose,
		// because our C code only sets Lastexpr() on successful evaluation.
		//
		// We could always move it later, after the did error check,
		// if that does pop up in the future.
		embedr.EvalR(`sink(file=NULL)`)

		if did == 0 {
			// simple error
			continue
		}
		if did < 0 {
			// ctrl-d (EOF or end-of-file); back when using readline anyway.
			// Ask the user if they want to quit, just as usual.
			embedr.EvalR(`q()`)
			continue
		}
		cmd := strings.TrimSpace(embedr.Lastexpr())
		//vv("cmd = '%v'", cmd)

		if cmd == "" {
			continue
		}

		// weed out the ess crap
		if strings.HasPrefix(cmd, ".ess") ||
			strings.Contains(cmd, "options(STERM") ||
			strings.Contains(cmd, ".emacs.d/ESS/etc/ESSR") {

			// ignore the garbage .ess_funargs stuff
			continue
		}

		var e = &HashRElem{
			Tm:    time.Now(),
			Seqno: seqno,
		}
		switch {

		case strings.HasPrefix(cmd, "dv("):
			//dvvFunc()
			//continue // dvvFunc() does the CODEX code below; it has to for browser to see the console output.

			if capturedOutputOK && prevJSON != "" {

				//vv("prevJSON = '%v'; prevJSON2 = '%v'", prevJSON, prevJSON2)

				prev := prevJSON
				if isGarbage(prevJSON) {
					// more injected ESS garbage?
					// try one further back. Yes this works, at least in the one time we saw.
					//vv("trying prevJSON2='%v' instead of prevJSON='%v'", prevJSON2, prevJSON)
					prev = prevJSON2
				}

				msg := prepConsoleMessage(prev, seqno)
				e.Typ = Console
				e.ConsoleJSON = msg
				e.msg = []byte(msg)

				// append to our text file version on disk
				script = writeScriptConsole(script, prevCaptureOK)

				hub.broadcast <- e
				seqno++
				archiveElem(e)
			}
		case cmd == "sv()":
			svvPlot()
			continue // svvPlot() does the archiveElem(); it has to for browser to see the plot.
		default:

			// special handling for strings literal values
			// that start with `"#` or `;`. We present them
			// as comments in the rbook browser view.
			//
			// # is the traditional comment start, while '; is very easy to
			// type to start a comment, as no shift key is required. Simply
			// add the comment, then end the string literal with another
			// single quote '.
			//
			// We also take advantage of the observation that the R parser
			// will reject actual commands starting
			// with semicolons, so there can't be any confusion here between
			// a command and a string literal. We know
			// we are examining a last expression value, so we know it got
			// parsed just fine, and thus it must be a string literal. If
			// the string starts with a semicolon we know; it is our comment.
			//
			// > ; print("hi")
			// Error: unexpected ';' in ";"
			// >
			//
			if strings.HasPrefix(cmd, `"#`) || strings.HasPrefix(cmd, `";`) {

				//vv("see comment: '%v'", cmd)

				msg := prepCommentMessage(cmd, seqno)
				e.Typ = Comment
				e.CommentJSON = msg
				e.msg = []byte(msg)

				script = writeScriptComment(script, cmd)

				hub.broadcast <- e
				seqno++
				archiveElem(e)

			} else { // cmd

				msg, numlines := prepCommandMessage(cmd, seqno)
				e.Typ = Command
				e.CmdJSON = msg
				e.msg = []byte(msg)
				e.BeginCommandLineNum = lastCommandLineNum + 1
				e.NumCommandLines = numlines
				lastCommandLineNum += numlines

				script = writeScriptCommand(script, cmd, e.BeginCommandLineNum, e.Tm)

				hub.broadcast <- e
				//vv("send cmd='%v' as seqno = %v", cmd, seqno)
				seqno++
				archiveElem(e)

				//vv("autoDV = %v, at cmd = '%v'", autoDV, cmd)
				if autoDV {
					// reject progress messages from in-progress operations
					isProgress := strings.Contains(captureJSON, "|=") ||
						strings.Contains(captureJSON, "|--") // or "|======", ...

					// version of dv() that does not need to use prev and prevCaptureOK
					if capturedOutputOK && captureJSON != "" && !isProgress {

						// before we overwrite e from the cmd just above,
						// save it as in the CODEX.

						history.mut.Lock()
						history.elems = append(history.elems, e)
						history.mut.Unlock()

						by, err := e.SaveToSlice()
						panicOn(err)
						_, err = appendFD.Write(by)
						panicOn(err)

						// ship capture
						//vv("autoDV is on. shipping captureJSON = '%v'", captureJSON)

						//vv("prevJSON = '%v'; prevJSON2 = '%v'", prevJSON, prevJSON2)

						msg := prepConsoleMessage(captureJSON, seqno)
						// do not reuse e, possible race with shipping it to the browser

						var e2 = &HashRElem{
							Tm:    e.Tm,
							Seqno: seqno,
						}

						e2.Typ = Console
						e2.ConsoleJSON = msg
						e2.msg = []byte(msg)

						// append to our text file version on disk
						script = writeScriptConsole(script, captureOK)

						hub.broadcast <- e2
						seqno++
						archiveElem(e2)
					}

					// auto sv() too
					if strings.HasPrefix(cmd, "plot(") || strings.HasPrefix(cmd, "hist(") {
						svvPlot()
					}
				} // end if autoDV
			} // end else cmd
		} // end switch
	}
	select {}
}

func escape(s string) (res string, grew int) {
	if len(s) == 0 {
		return
	}

	beglen := len(s)

	// coerce any control characters to be valid JSON
	by, err := json.Marshal(s)
	panicOn(err)

	// remove the double quotes added at begin/end, since
	// we prepend some `"## ` stuff and manually add the double quotes.
	if by[0] == '"' {
		by = by[1:]
	}
	n := len(by)
	if by[n-1] == '"' {
		by = by[:n-1]
	}
	res = string(by)
	grew = len(res) - beglen
	return
}

// add length: as prefix, so we can parse 2 messages that get piggy backed,
// as occassionally happens on the websockets.
func prepCommandMessageOld(msg string, seqno int) string {
	if msg == "" {
		return ""
	}
	esc, grew := escape(msg)
	_ = grew
	json := fmt.Sprintf(`{"seqno": %v, "command":"%v"}`, seqno, esc)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

// new version of prepCommandMessage that, like prepCommentMessage, doesn't compress into one line
// As with all now, add length: as prefix, so we can parse 2 messages that get piggy backed,
// as occassionally happens on the websockets.
func prepCommandMessage(msg string, seqno int) (jsonstring string, numlines int) {
	//n := len(msg)
	//msg = msg[1 : n-1]

	//vv("prepCommandMessage msg = '%#v'", msg)

	// one line into possibly multiple lines
	commands := strings.Split(msg, "\n")

	//vv("commands = '%#v'", commands)

	// get a json array of string
	by, err := json.Marshal(commands)
	panicOn(err)

	commandsJSON := string(by)

	//vv("commandsJSON = '%#v'", commandsJSON)

	json := fmt.Sprintf(`{"seqno": %v, "command":%v}`, seqno, commandsJSON)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson, len(commands)
}

func prepCommentMessage(msg string, seqno int) string {
	if msg == "" {
		return ""
	}

	// we know msg starts with `"#`, and ends with `"`;
	// strip those off for the moment
	n := len(msg)
	msg = msg[2 : n-1]

	// one line into possibly multiple lines
	lines := strings.Split(msg, "\\n")
	//vv("lines = '%#v'", lines)
	var comments []string
	for _, line := range lines {
		// since we do json.Marshal() below, calling escape() as well is
		// double escaping  > and < ; not needed and makes comments garbled.
		//escline := escape(line)
		//vv("line '%v' -> escline '%v'", line, escline)
		//comments = append(comments, `### `+escline)

		comments = append(comments, `### `+line)
	}

	// get a json array of string
	by, err := json.Marshal(comments)
	panicOn(err)

	commentsJSON := string(by)

	//vv("commentsJSON = '%#v'", commentsJSON)

	json := fmt.Sprintf(`{"seqno": %v, "comment":%v}`, seqno, commentsJSON)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

func prepConsoleMessage(consoleOut string, seqno int) string {
	if consoleOut == "" {
		return ""
	}
	json := fmt.Sprintf(`{"seqno": %v, "console":%v}`, seqno, consoleOut)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

func prepOverlayLaterNoteMessage(note string, seqno, overlayOnSeqno int) string {
	if note == "" {
		return ""
	}
	escNote, grew := escape(note)
	_ = grew

	json := fmt.Sprintf(`{"seqno": %v, "overlayNote":"%v", "overlayOnSeqno":%v}`, seqno, escNote, overlayOnSeqno)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

func prepOverlayHideOutput(seqno, hideSeqno int) string {
	json := fmt.Sprintf(`{"seqno": %v, "overlayHideSeqno":%v}`, seqno, hideSeqno)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

func prepImageMessage(path, pathhash string, seqno int) string {
	if path == "" {
		return ""
	}
	json := fmt.Sprintf(`{"seqno":%v, "image":"%v", "pathhash":"%v"}`, seqno, path, pathhash)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

// book.mut must be held by caller
func prepInitMessage(book *HashRBook) string {
	// don't want to send the elements

	by, err := json.Marshal(book)
	panicOn(err)

	json := fmt.Sprintf(`{"init":true, "book":%v}`, string(by))
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return lenPrefixedJson
}

func writeScriptComment(script *os.File, msg string) *os.File {

	if msg == "" {
		return script
	}
	n := len(msg)
	msg = msg[2 : n-1]

	// one line into possibly multiple lines
	lines := strings.Split(msg, "\\n")
	//vv("lines = '%#v'", lines)
	for _, line := range lines {
		fmt.Fprintf(script, "### %v\n", line)
	}

	return script
}

// 60 spaces to move the command line comments off to the right of the screen. Less distracting.
// Now 52 to make room for the timestamp too.
var spacer string = strings.Repeat(" ", 52)

func writeScriptCommand(script *os.File, cmd string, linenum int, at time.Time) *os.File {
	fmt.Fprintf(script, spacer+" ## command line [%03d]: %v\n%v\n", linenum, at.In(Chicago).Format(RFC3339MicroNumericTZ), cmd)
	return script
}

func writeScriptImage(script *os.File, path string) *os.File {
	fmt.Fprintf(script, "    ##img=readPNG('%v');x11();grid::grid.raster(img); #saved\n", path)
	return script
}

func writeScriptConsole(script *os.File, prevCaptureOK []string) *os.File {
	for _, line := range prevCaptureOK {
		fmt.Fprintf(script, "    ## %v\n", line)
	}
	return script
}

type DecodeJSON struct {
	Seqno   int      `json:"seqno"`
	Command []string `json:"command"`
	Console []string `json:"console"`
	Comment []string `json:"comment"`
	Image   string   `json:"image"`
}

func (c *RbookConfig) dumpToScript(fd *os.File, book *HashRBook) {

	// try to dedup commands, can have repeated command lines in our history.
	lastCommandLineNum := 0

	for i, e := range book.elems {
		colon := bytes.Index(e.msg, []byte{':'})
		msg := e.msg[colon+1:]
		d := &DecodeJSON{}
		err := json.Unmarshal(msg, d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "problem at i = %v, colon = %v, e = '%#v': msg='%v', err = '%v'", i, colon, e, string(msg), err)
			panicOn(err)
		}

		if c.DumpTimestamps {
			extra := ""
			if e.BeginCommandLineNum > 0 {
				extra = fmt.Sprintf("command line [%03d] ", e.BeginCommandLineNum)
			}
			fmt.Printf("          ##  ===== %v %v =====:\n", e.Tm.In(Chicago).Format(RFC3339MicroNumericTZ), extra)
		} else {
			// match what incrementally appended .rsh looks like, so we can
			// re-create on git rebase deletion.
			if e.BeginCommandLineNum > 0 {
				if e.BeginCommandLineNum == lastCommandLineNum {
					// eliminate duplicates
					continue
				}
				lastCommandLineNum = e.BeginCommandLineNum
				// keep this matching the writeScriptCommand() output at rbook.go:1203
				fmt.Fprintf(fd, spacer+" ## command line [%03d]: %v\n", e.BeginCommandLineNum, e.Tm.In(Chicago).Format(RFC3339MicroNumericTZ))
			}
		}
		switch e.Typ {
		case Command:
			for _, line := range d.Command {
				fmt.Fprintf(fd, "%v\n", line)
			}
		case Comment:
			for _, line := range d.Comment {
				fmt.Fprintf(fd, "%v\n", line)
			}
		case Console:
			for _, line := range d.Console {
				fmt.Fprintf(fd, "    %v\n", line)
			}
		case Image:
			fmt.Fprintf(fd, "    ##img=readPNG('%v');x11();grid::grid.raster(img); #saved\n", d.Image)
		}

	}

	fd.Sync()
}

// where we continue our command line numbering from
func getLastCommandLineNum(history *HashRBook) (lastCommandLineNum int) {
	if history == nil {
		return 0
	}
	n := len(history.elems)
	if n == 0 {
		return 0
	}
	// since only the commands are numbered, we have
	// to locate the last one.
	for i := n - 1; i >= 0; i-- {
		e := history.elems[i]
		if e.Typ == Command {
			return e.LastCommandLineNumber()
		}
	}
	// no commands yet in this history (weird but oh well).
	return 0
}

// annoying garbage that ess can auto inject... we attempt skip it/remove it
// from the rbook command line stream.
var annoyances = []string{`if (identical(getOption('pager'),`, // ... file.path(R.home('bin'), 'pager'))) options(pager='cat') # rather take the ESS one`
	"local({\n",
	//...
	//    source("/Users/jaten/.emacs.d/ESS-17.11/etc/ESSR/R/.load.R", local = TRUE)
	//    load.ESSR("/Users/jaten/.emacs.d/ESS-17.11/etc/ESSR/R")
	//})`
	`(list \"\" '((\"`,
}

var essGarbage string = `(list \"\" '((\"` // randomly injected by ESS, ignored by rbook.

func isGarbage(s string) bool {
	if strings.Contains(s, essGarbage) {
		return true
	}
	for i := range annoyances {
		if strings.HasPrefix(s, annoyances[i]) {
			return true
		}
	}
	return false
}

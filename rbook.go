package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/glycerine/blake2b-simd"
	"github.com/glycerine/cryrand"
	"github.com/glycerine/embedr"
)

var _ = syscall.Getpgid

const RFC3339NanoNumericTZ0pad = "2006-01-02T15:04:05.000000000-07:00"

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

func intercept_SIGINT() {
	//vv("intercept_SIGINT installing")
	c := make(chan os.Signal, 100)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, ctrl-c, handle it
			_ = sig
			//fmt.Printf("go/rbook squashed SIGINT\n")
		}
	}()
}

// avoid leaving dangling Xvfb/x11vnc/icewm when we kill the *R*
// session from within emacs.
func (cfg *RbookConfig) intercept_SIGTERM_and_cleanup() {
	ch := make(chan os.Signal, 100)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		<-ch
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

	fn := cfg.RbookFilePath
	cwd, err := os.Getwd()
	panicOn(err)
	bookpath := cwd + sep + fn

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
	if cfg.Dump {
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

	// write header for the script
	if freshScript {
		fmt.Fprintf(script, `#!/bin/bash
exec R --vanilla -q --slave -e "source(file=pipe(\"tail -n +3 $0\"))" --args $@

# text version of:

#%v@%v:%v
#BookID:%v
#R rbook created: %v

require(png)

`, username, hostname, bookpath, history.BookID, history.CreateTm.Format(RFC3339NanoNumericTZ0pad))
	}

	if cfg.Dump {
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

	disp := GetAvailXvfbDisplay()
	display := fmt.Sprintf(":%v", disp)
	os.Setenv("DISPLAY", display)
	vncPort := cfg.StartXvfbAndFriends(display)
	fmt.Printf("Xvfb using DISPLAY=:%v  R_HOME=%v  vncPort=%v\n", disp, cfg.Rhome, vncPort)

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

	StartShowme(cfg, history) // serve the initial html and the png files to the web browsers
	//vv("Showme http server started. Starting reload websocket server.")
	cfg.startReloadServer(history) // websockets to tell browsers what to show when there's an update.
	//vv("Reload server started.")

	// number the saved png files.
	nextSave := 0

	// our repl
	embedr.ReplDLLinit()
	embedr.SetGoCallbackForCleanup(func() { cfg.StopXvfb() })

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

	// need to save one console capture back for dv() recording of output
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
	essGarbage := `(list \"\" '((\"` // randomly injected by ESS, ignored by rbook.

	for {

		//updatePromptCwd("")
		embedr.EvalR(`if(exists("zrecord_mini_console")) { rm("zrecord_mini_console") }`)
		embedr.EvalR(`sink(textConnection("zrecord_mini_console", open="w"), split=T);`)

		path := ""
		did := embedr.ReplDLLdo1()
		_ = did
		//vv("did = %v", did)
		if did > 1 {
			// did == 2: this seems to mean that the call is incomplete;
			//vv("back from one call to R_ReplDLLdo1(); did = %v\n", did)
		}
		// did == 0 => error evaluating
		// did == -1 => ctrl-d (end of file).

		sinkgot, err := embedr.EvalR_fullback(`zrecord_mini_console`)
		panicOn(err)
		capture, capturedOutputOK := sinkgot.([]string)

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
				if strings.Contains(line, essGarbage) {
					continue
				}
				newlines += line + "\n"
				esc, grew := escape(line)
				if grew > 0 {
					vv("see grew = %v on line '%v'", line)
					vv("esc version = '%v'", esc)
				}
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
			if capturedOutputOK && prevJSON != "" {

				//vv("prevJSON = '%v'; prevJSON2 = '%v'", prevJSON, prevJSON2)

				prev := prevJSON
				if strings.Contains(prevJSON, essGarbage) {
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
				writeScriptConsole(script, prevCaptureOK)

				hub.broadcast <- e
				seqno++
			}
		case cmd == "sv()":
			odir := bookpath + ".plots"
			panicOn(os.MkdirAll(odir, 0777))
			rnd20 := cryrand.RandomStringWithUp(20)
			path = fmt.Sprintf("%v/plotmini_%03d_%v.png", odir, nextSave, rnd20)
			err := embedr.EvalR(fmt.Sprintf(`savePlot(filename="%v")`, path))
			if err != nil {
				// possibly "no plot on device to save";
				// don't bother to send to browser. And don't crash.
				continue
			}
			panicOn(err)
			pathhash, imageby := PathHash(path)
			//vv("saved to path = '%v'; pathhash='%v'", path, pathhash)
			nextSave++

			//vv("Reloading browser with image path '%v'", path)
			msg := prepImageMessage(path, pathhash, seqno)

			e.Typ = Image
			e.ImageJSON = msg
			e.ImageHost = hostname
			e.ImagePath = path
			e.ImageBy = imageby
			e.ImagePathHash = pathhash
			e.msg = []byte(msg)

			writeScriptImage(script, path)

			hub.broadcast <- e
			seqno++
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

				writeScriptComment(script, cmd)

				hub.broadcast <- e
				seqno++

			} else {

				msg := prepCommandMessage(cmd, seqno)
				e.Typ = Command
				e.CmdJSON = msg
				e.msg = []byte(msg)

				writeScriptCommand(script, cmd)

				hub.broadcast <- e
				seqno++
			}
		}

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
func prepCommandMessage(msg string, seqno int) string {
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
	return lenPrefixedJson
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

func writeScriptComment(script *os.File, msg string) {

	if msg == "" {
		return
	}
	n := len(msg)
	msg = msg[2 : n-1]

	// one line into possibly multiple lines
	lines := strings.Split(msg, "\\n")
	//vv("lines = '%#v'", lines)
	for _, line := range lines {
		fmt.Fprintf(script, "### %v\n", line)
	}
}

func writeScriptCommand(script *os.File, cmd string) {
	fmt.Fprintf(script, "%v\n", cmd)
}

func writeScriptImage(script *os.File, path string) {
	fmt.Fprintf(script, "    ##img=readPNG('%v');x11();grid::grid.raster(img); #saved\n", path)
}

func writeScriptConsole(script *os.File, prevCaptureOK []string) {
	for _, line := range prevCaptureOK {
		fmt.Fprintf(script, "    ## %v\n", line)
	}
}

type DecodeJSON struct {
	Seqno   int      `json:"seqno"`
	Command []string `json:"command"`
	Console []string `json:"console"`
	Comment []string `json:"comment"`
	Image   string   `json:"image"`
}

func (c *RbookConfig) dumpToScript(fd *os.File, book *HashRBook) {
	for i, e := range book.elems {
		colon := bytes.Index(e.msg, []byte{':'})
		msg := e.msg[colon+1:]
		d := &DecodeJSON{}
		err := json.Unmarshal(msg, d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "problem at i = %v, colon = %v, e = '%#v': msg='%v', err = '%v'", i, colon, e, string(msg), err)
			panicOn(err)
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
				fmt.Fprintf(fd, "   %v\n", line)
			}
		case Image:
			fmt.Fprintf(fd, "    ##img=readPNG('%v');x11();grid::grid.raster(img); #saved\n", d.Image)
		}

	}

	fd.Sync()
}

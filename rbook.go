package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/glycerine/blake2b-simd"
	"github.com/glycerine/cryrand"
	"github.com/glycerine/embedr"
)

func init() {
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
//
func PathHash(path string) (hash string, imageBy []byte) {
	hasher.Reset()
	hasher.Write([]byte(hostname + ":" + path + ":"))
	by, err := ioutil.ReadFile(path)
	panicOn(err)
	hasher.Write(by)
	return base64.RawURLEncoding.EncodeToString(hasher.Sum(nil)), by
}

func main() {

	cfg := &RbookConfig{}
	myflags := flag.NewFlagSet("myflags", flag.ExitOnError)
	cfg.DefineFlags(myflags)

	err := myflags.Parse(os.Args[1:])
	err = cfg.ValidateConfig(myflags)
	if err != nil {
		AlwaysPrintf("%s command line flag error: '%s'", ProgramName, err)
		os.Exit(1)
	}

	fn := cfg.RbookFilePath
	cwd, err := os.Getwd()
	panicOn(err)
	bookpath := cwd + sep + fn

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

	// for each new websocket client, as they
	// arrive, replay the history for them.

	// As we enter new commands, create new console output,
	// and generate images, save each to the history struct
	// and to the file.

	// start R first so it gets the main thread.

	// TODO: start up Xvfb on a free DISPLAY like :99
	// Xvfb :99 -screen 0 3000x2000x16
	// icewm &
	// feh --bg-scale ~/pexels-ian-turnell-709552.jpg
	// x11vnc -display :99 -forever -nopw -quiet -xkb

	// For this proof-of-principle, these have already
	// been started manually.

	os.Setenv("DISPLAY", ":99")
	os.Setenv("R_HOME", "/usr/lib/R")
	embedr.InitR(true)
	defer embedr.EndR()
	//embedr.EvalR("x11(); hist(rnorm(1000))") // only did the x11(); did not hist()
	//embedr.EvalR("require(R.utils)") // for captureOutput()
	embedr.EvalR("x11()")

	//embedr.EvalR("hist(rnorm(1000))")             // worked.
	//embedr.EvalR(`savePlot(filename="hist.png")`) // worked.
	// nice to have a starting .png so showme does not crash :)
	// not to worry now. We have logo and 2 favicons always.

	// If you want a custom prompt...
	// https://lapsedgeographer.london/2020-11/custom-r-prompt/  says:
	//
	// "The prompt is a simple character string, stored in .Options, meaning we
	// can easily inspect it and modify it.  You can even use emoji, though
	// I’d recommend using a base emoji rather than a composite emoji...
	//
	// > getOption("prompt")
	// [1] "> "
	//	> options("prompt" = "! ")
	//	!

	// don't need to hold mut b/c reload server not started yet
	seqno := len(history.elems)

	StartShowme(cfg) // serve the initial html and the png files to the web browsers
	//vv("Showme http server started. Starting reload websocket server.")
	startReloadServer(history) // websockets to tell browsers what to show when there's an update.
	//vv("Reload server started.")

	// number the saved png files.
	nextSave := 0

	// our repl
	embedr.ReplDLLinit()
	// cannot do invisible(TRUE) inside here; as that will
	// hide the previous command output!
	embedr.EvalR(`sv=function(){}`) // easy to type. cmd == "sv()" tells us to save the current graph.
	embedr.EvalR(`dv=function(){}`) // easy to type. cmd == "dv()" tells us to save the last value.

	// need to save one console capture back for dv() recording of output
	captureJSON := ""
	prevJSON := ""
	for {

		embedr.EvalR(`if(exists("zrecord_mini_console")) { rm("zrecord_mini_console") }`)
		embedr.EvalR(`sink(textConnection("zrecord_mini_console", open="w"), split=T);`)

		path := ""
		did := embedr.ReplDLLdo1()
		_ = did
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
			prevJSON = captureJSON
			captureJSON = "["

			//vv("capture = %v lines\n", len(capture))
			for i, line := range capture {
				//fmt.Printf("line %02d: %v\n", i, line)
				if i == 0 {
					captureJSON += fmt.Sprintf(`"## %v"`, escape(line))
				} else {
					captureJSON += fmt.Sprintf(`,"## %v"`, escape(line))
				}
			}
			captureJSON += `]`
		}
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
			// ctrl-d (EOF or end-of-file).
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
		if strings.HasPrefix(cmd, ".ess") {
			// ignore the garbage .ess_funargs stuff
			continue
		}

		var e = &HashRElem{
			Tm:    time.Now(),
			Seqno: seqno,
		}
		switch cmd {
		case "dv()":
			if capturedOutputOK && prevJSON != "" {

				msg := prepConsoleMessage(prevJSON, seqno)
				e.Typ = Console
				e.ConsoleJSON = msg
				e.msg = []byte(msg)

				hub.broadcast <- e
				seqno++
			}
		case "sv()":
			odir := ".rbook"
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

			hub.broadcast <- e
			seqno++
		default:

			// special handling for strings literal values
			// that start with `"#`. We present them
			// as comments in the rbook browser view.
			if strings.HasPrefix(cmd, `"#`) {

				//vv("see comment: '%v'", cmd)

				msg := prepCommentMessage(cmd, seqno)
				e.Typ = Comment
				e.CommentJSON = msg
				e.msg = []byte(msg)

				hub.broadcast <- e
				seqno++

			} else {

				msg := prepCommandMessage(cmd, seqno)
				e.Typ = Command
				e.CmdJSON = msg
				e.msg = []byte(msg)
				hub.broadcast <- e
				seqno++
			}
		}

		history.mut.Lock()
		history.elems = append(history.elems, e)
		history.mut.Unlock()

		by, err := e.SaveToSlice()
		panicOn(err)
		_, err = appendFD.Write(by)
		panicOn(err)
	}
	select {}
}

func escape(s string) string {
	if len(s) == 0 {
		return s
	}

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
	return string(by)
}

// add length: as prefix, so we can parse 2 messages that get piggy backed,
// as occassionally happens on the websockets.
func prepCommandMessage(msg string, seqno int) string {
	if msg == "" {
		return ""
	}
	json := fmt.Sprintf(`{"seqno": %v, "command":"%v"}`, seqno, escape(msg))
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
		escline := escape(line)
		//vv("line '%v' -> escline '%v'", line, escline)

		comments = append(comments, `### `+escline)
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

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/glycerine/blake2b-simd"
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
}

var hostname string
var hasher hash.Hash

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

	bookpath := "my.hashr.book"

	var history *HasherBook

	history, appendFD, err := ReadBook(bookpath)
	panicOn(err)
	vv("see history len %v:", len(history.Elems))
	for _, e := range history.Elems {
		fmt.Printf("%v\n", e)
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

	embedr.EvalR("hist(rnorm(1000))")             // worked.
	embedr.EvalR(`savePlot(filename="hist.png")`) // worked.
	// nice to have a starting .png so showme does not crash :)

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

	StartShowme() // serve the initial html and the png files to the web browsers
	//log.Println("Showme http server started. Starting reload websocket server.")
	startReloadServer() // websockets to tell browsers what to show when there's an update.
	//log.Println("Reload server started.")

	// number the saved png files.
	nextSave := 0

	// our repl
	embedr.ReplDLLinit()
	// cannot do invisible(TRUE) inside here; as that will
	// hide the previous command output!
	embedr.EvalR(`sv=function(){}`) // easy to type. cmd == "sv()" tells us to save the current graph.
	embedr.EvalR(`dv=function(){}`) // easy to type. cmd == "dv()" tells us to save the last value.

	seqno := len(history.Elems)

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

		var e = &HasherElem{
			Tm:    time.Now(),
			Seqno: seqno,
		}
		switch cmd {
		case "dv()":
			if capturedOutputOK && prevJSON != "" {

				msg := prepConsoleMessage(prevJSON, seqno)
				e.Typ = Console
				e.ConsoleJSON = msg
				hub.broadcast <- []byte(msg)
				seqno++
			}
		case "sv()":
			path = fmt.Sprintf("plotmini_%03d.png", nextSave)
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

			hub.broadcast <- []byte(msg)
			seqno++
		default:
			msg := prepCommandMessage(cmd, seqno)
			e.Typ = Command
			e.CmdJSON = msg
			hub.broadcast <- []byte(msg)
			seqno++
		}
		history.Elems = append(history.Elems, e)
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

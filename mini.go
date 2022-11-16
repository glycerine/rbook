package main

import (
	//"bufio"
	//"bytes"
	"fmt"
	//"log"
	"os"
	"runtime"
	"strings"
	//"time"

	"github.com/glycerine/embedr"
	//"github.com/glycerine/rmq"
)

func init() {
	// Arrange that main.main runs on main thread. Hopefully this helps R startup.
	runtime.LockOSThread()
}

func main() {

	// start R first so maybe it gets the main thread.

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
	embedr.EvalR("require(R.utils)") // for captureOutput()
	embedr.EvalR("x11()")

	embedr.EvalR("hist(rnorm(1000))")             // worked.
	embedr.EvalR(`savePlot(filename="hist.png")`) // worked.
	// nice to have a starting .png so showme does not crash :)

	// https://lapsedgeographer.london/2020-11/custom-r-prompt/
	//The prompt is a simple character string, stored in .Options, meaning we
	// can easily inspect it and modify it.  You can even use emoji, though
	// I’d recommend using a base emoji rather than a composite emoji...
	//
	// > getOption("prompt")
	// [1] "> "
	//	> options("prompt" = "! ")
	//	!

	//vv("done with eval")

	StartShowme() // serve the initial html and the png files to the web browsers
	//log.Println("Showme http server started. Starting reload websocket server.")
	startReloadServer() // websockets to tell browsers what to show when there's an update.
	//log.Println("Reload server started.")

	// number the saved png files.
	nextSave := 0

	// our repl
	embedr.ReplDLLinit()
	embedr.EvalR(`sv=function(){}`) // easy to type. cmd == "sv()" tells us to save the current graph.

	seqno := 0
	for {
		path := ""
		did := embedr.ReplDLLdo1()
		_ = did
		//vv("back from one call to R_ReplDLLdo1(); did = %v\n", did)
		// did == 0 => error evaluating
		// did == -1 => ctrl-d (end of file).

		//if did <= 0 {
		//	break
		//}
		if did < 1 {
			// error, keep going. also ctrl-d, EOF
			//embedr.EvalR(`q()`)
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

		if cmd == "sv()" {
			path = fmt.Sprintf("plotmini_%03d.png", nextSave)
			err := embedr.EvalR(fmt.Sprintf(`savePlot(filename="%v")`, path))
			panicOn(err)
			nextSave++

			//vv("Reloading browser with image path '%v'", path)
			hub.broadcast <- prepImageMessage(path, seqno)
			seqno++
		} else {
			hub.broadcast <- prepTextMessage(cmd, seqno)
			seqno++
		}
	}
	select {}
}

// add length: as prefix, so we can parse 2 messages that get piggy backed.
func prepTextMessage(msg string, seqno int) []byte {
	if msg == "" {
		return nil
	}
	escaped := strings.ReplaceAll(msg, `"`, `\"`)
	json := fmt.Sprintf(`{"seqno": %v, "text":"%v"}`, seqno, escaped)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return []byte(lenPrefixedJson)
}

func prepImageMessage(path string, seqno int) []byte {
	if path == "" {
		return nil
	}
	json := fmt.Sprintf(`{"seqno":%v, "image":"%v"}`, path, seqno)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return []byte(lenPrefixedJson)
}

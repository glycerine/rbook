package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	//"time"

	"github.com/glycerine/embedr"
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
	embedr.InitR()
	defer embedr.EndR()
	//embedr.EvalR("x11(); hist(rnorm(1000))") // only did the x11(); did not hist()
	embedr.EvalR("require(R.utils)") // for captureOutput()
	embedr.EvalR("x11()")
	embedr.EvalR("hist(rnorm(1000))") // worked.
	vv("done with eval")

	embedr.EvalR(`savePlot(filename="hist.png")`) // worked.

	StartShowme()

	log.Println("Starting reload server.")

	startReloadServer()

	log.Println("Reload server started.")

	log.Println("Press Enter to reload the browser!")
	nextSave := 0
	for {
		reader := bufio.NewReader(os.Stdin)
		expr, err := reader.ReadString('\n')
		panicOn(err)
		vv("expr = '%v'", expr)
		cmd := strings.TrimSpace(expr)
		path := ""
		if cmd == "save" {
			path = fmt.Sprintf("hist_%03d.png", nextSave)
			err := embedr.EvalR(fmt.Sprintf(`savePlot(filename="%v")`, path))
			panicOn(err)
			nextSave++
		} else {
			// doesn't work to get back output:
			//capture := fmt.Sprintf("___cap = captureOutput(%v)", expr)
			//ev, err := embedr.EvalR(capture)
			//panicOn(err)
			//vv("ev = '%#v'", ev)
			//output, err := embedr.EvalR("___cap")
			//panicOn(err)
			//vv("output = '%#v'", output)

			err := embedr.EvalR(expr)
			if err != nil {
				fmt.Printf("%v\n", err)
			}

			hub.broadcast <- prepTextMessage(cmd)

			if err != nil {
				// heh. 100msec sleep prevents websocket from
				// concatenating our messages... they need prepended lengths of messages!
				//time.Sleep(100 * time.Millisecond)
				hub.broadcast <- prepTextMessage(err.Error())
				//vv("sent error '%v' as text", sending)
			}

			//message := bytes.TrimSpace([]byte(fmt.Sprintf(`{"text":"%v"}`, strings.ReplaceAll(strOut, `"`, `\"`))))
			//hub.broadcast <- message
		}

		if path != "" {
			log.Println("Reloading browser.")
			//sendReload()

			hub.broadcast <- prepImageMessage(path)
		}
	}

	message := bytes.TrimSpace([]byte(`{"image":"hist.png"}`))
	hub.broadcast <- message

	select {}
}

// add length: as prefix, so we can parse 2 messages that get piggy backed.
func prepTextMessage(msg string) []byte {
	if msg == "" {
		return nil
	}
	escaped := strings.ReplaceAll(msg, `"`, `\"`)
	json := fmt.Sprintf(`{"text":"%v"}`, escaped)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return []byte(lenPrefixedJson)
}

func prepImageMessage(path string) []byte {
	if path == "" {
		return nil
	}
	json := fmt.Sprintf(`{"image":"%v"}`, path)
	lenPrefixedJson := fmt.Sprintf("%v:%v", len(json), json)
	return []byte(lenPrefixedJson)
}

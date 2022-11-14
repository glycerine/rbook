package main

import (
	"bufio"
	"log"
	"os"

	"github.com/glycerine/embedr"
)

func main() {

	if false {
		log.Println("Starting reload server.")

		startReloadServer()

		log.Println("Reload server started.")
		log.Println("Press Enter to reload the browser!")
		for {
			reader := bufio.NewReader(os.Stdin)
			expr, err := reader.ReadString('\n')
			panicOn(err)
			vv("expr = '%v'", expr)

			log.Println("Reloading browser.")
			sendReload()
		}

	}

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
	embedr.EvalR("x11()")
	embedr.EvalR("hist(rnorm(1000))") // worked.
	vv("done with eval")

	embedr.EvalR(`savePlot(filename="hist.png")`) // worked.

	select {}
}

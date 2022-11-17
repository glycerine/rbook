package main

import (
	//"flag"
	"bytes"
	"fmt"
	html_template "html/template"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	//"github.com/glycerine/fsnotify"
	//"github.com/skratchdot/open-golang/open"
)

/*
present a directory of png images to
a web page that makes it easy to
one-click through them
*/

var ProgramName, Cmdline string

var _ = exec.Command

func StartShowme(cfg *RbookConfig) {

	ProgramName = path.Base(os.Args[0])
	Cmdline = strings.Join(os.Args, " ")

	// instantiate index.template -> index.html
	// with our websocket ports.
	var readyIndexHtmlBuf bytes.Buffer
	tmpl, err := html_template.ParseFiles("index.template")
	panicOn(err)

	vv("cfg = '%#v'", cfg)
	err = tmpl.Execute(&readyIndexHtmlBuf, cfg)
	panicOn(err)

	//vv("readyIndexHtmlBuf = '%v'\n", readyIndexHtmlBuf.String())

	pngs, err := filepath.Glob("*.png")
	panicOn(err)
	if len(pngs) == 0 {
		fmt.Fprintf(os.Stderr, "no png files present.\n")
		os.Exit(1)
	}
	cwd, err := os.Getwd()
	panicOn(err)
	_ = cwd
	//fmt.Printf("showme running in '%s' with %v png files\n", cwd, len(pngs))

	/*
			watcher, err := fsnotify.NewWatcher()
			panicOn(err)
			err = watcher.Add(cwd)
			panicOn(err)
			defer watcher.Close()

		rescanNeeded := make(chan bool, 1)
		go func() {
		waitForWrite:
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						continue waitForWrite
					}
					select {
					case rescanNeeded <- true:
					default:
					}
					//fmt.Printf("event: '%v'\n", event)
					if event.Has(fsnotify.Write) {
						//fmt.Printf("modified file: '%v'\n", event.Name)

					}
					continue waitForWrite
				case err, _ := <-watcher.Errors:
					panicOn(err)

					//case <-c.haltTail.ReqStop.Chan:
					//	return
				}
			}
		}()
	*/

	http.Handle("/images/", http.StripPrefix("/images/",
		http.FileServer(http.Dir("."))))

	n := len(pngs)

	// don't crash if no png files; just don't bother with the /view functionality
	// TODO: in the future, maybe run a watch for .png files, and if they
	// show up then start the /view handler.
	viewOff := true
	if n > 0 {
		viewOff = false

		order := make(map[string]int)
		for i := range pngs {
			order[pngs[i]] = i
		}
		cur := 0
		prev := 0
		next := 0
		if n > 1 {
			next = 1
		}
		curpng := pngs[cur]
		prevpng := pngs[prev]
		nextpng := pngs[next]

		viewHandler := func(w http.ResponseWriter, r *http.Request) {
			what := r.URL.Path // [1:]
			if strings.HasSuffix(what, ".png") {
				curpng = path.Base(what)
			}

			loc := order[curpng]
			switch {
			case n == 1:
				prevpng = curpng
				nextpng = curpng
			case n == 2:
				if loc == 0 {
					prevpng = pngs[1]
					nextpng = pngs[1]
				} else {
					// loc == 1
					prevpng = pngs[0]
					nextpng = pngs[0]
				}
			default:
				// n >= 3
				if loc == 0 {
					// wrap around
					prevpng = pngs[n-1]
					nextpng = pngs[1]
				} else {
					nextpng = pngs[(loc+1)%n]
					prevpng = pngs[(loc-1)%n]
				}
			}
			//fmt.Printf("r.URL.Path='%#v'\n", r.URL.Path)

			fmt.Fprintf(w, `<html><head><script type = "text/JavaScript">`)
			script := fmt.Sprintf(`

		document.onkeydown = checkKey;

		function checkKey(event) {

			switch (event.key) {
			case "ArrowLeft":
				// Left pressed: previous image
                window.location.replace('/view/%v');
				break;
			case "ArrowRight":
				// Right pressed: next image
                window.location.replace('/view/%v');
				break;
			case "ArrowUp":
				// Up pressed
				break;
			case "ArrowDown":
				// Down pressed
				break;
			}
		}`, prevpng, nextpng)

			fmt.Fprintf(w, "%v</script></head><body>", script)
			fmt.Fprintf(w, `<font size="20">&nbsp;&nbsp;&nbsp;<a href="/view/%s">PREV</a>&nbsp;&nbsp;&nbsp;&nbsp;<a href="/view/%s">NEXT</a></font>&nbsp;[%03d&nbsp;of&nbsp;%03d]:&nbsp;%s<br>`, prevpng, nextpng, loc+1, n, curpng)
			fmt.Fprintf(w, `<a href="/view/%s"><img src="/images/%s"></a><br>`, nextpng, curpng)
			fmt.Fprintf(w, "</body></html>")
		}
		http.HandleFunc("/view/", viewHandler)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//http.ServeFile(w, r, "index.html")
		w.Write(readyIndexHtmlBuf.Bytes())
	})

	host := cfg.Host
	if host == "" {
		host = hostname // for nice presentation to the user.
	}
	fmt.Printf("\nUse http://%v:%v        -- for the rbook R session.\n", host, cfg.Port)
	if !viewOff {
		fmt.Printf("\nUse http://%v:%v/view   -- to view all .png images in initial directory.\n\n", host, cfg.Port)
	}
	go func() {
		err = http.ListenAndServe(fmt.Sprintf("%v:%v", cfg.Host, cfg.Port), nil)
		panicOn(err)
	}()
}

package main

import (
	//"flag"
	"fmt"
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

func StartShowme() {

	ProgramName = path.Base(os.Args[0])
	Cmdline = strings.Join(os.Args, " ")

	cfg := &ShowmeConfig{
		Port: 8080,
	}
	/* allow any R flags ess wants to set
	myflags := flag.NewFlagSet("myflags", flag.ExitOnError)
	cfg.DefineFlags(myflags)

	err := myflags.Parse(os.Args[1:])
	err = cfg.ValidateConfig()
	if err != nil {
		log.Fatalf("%s command line flag error: '%s'", ProgramName, err)
	}
	*/

	pngs, err := filepath.Glob("*.png")
	panicOn(err)
	if len(pngs) == 0 {
		fmt.Fprintf(os.Stderr, "no png files present.\n")
		os.Exit(1)
	}
	cwd, err := os.Getwd()
	panicOn(err)
	fmt.Printf("showme running in '%s' with %v png files\n", cwd, len(pngs))

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/view/", viewHandler)

	host := cfg.Host
	if host == "" {
		host = hostname
	}
	fmt.Printf("\nUse http://%v:%v        -- for the minibook R session.\n", host, cfg.Port)
	fmt.Printf("\nUse http://%v:%v/view   -- to view all .png images in initial directory.\n\n", host, cfg.Port)
	go func() {
		err = http.ListenAndServe(fmt.Sprintf("%v:%v", cfg.Host, cfg.Port), nil)
		panicOn(err)
	}()
	// hardcode darwin for now so that we can exit
	// automatically when safari is closed.
	//exec.Command("/usr/bin/open", "-F", "-W", "-n", fmt.Sprintf("http://%s", cfg.HostPort)).Run()

	//select {}
	//fmt.Printf("showme done.\n")
}

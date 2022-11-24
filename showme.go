package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	//"flag"
	"bytes"
	"fmt"
	html_template "html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
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

func StartShowme(cfg *RbookConfig, b *HashRBook) {

	ProgramName = path.Base(os.Args[0])
	Cmdline = strings.Join(os.Args, " ")

	// instantiate index.template -> index.html
	// with our websocket ports.
	var readyIndexHtmlBuf bytes.Buffer
	tmpl, err := html_template.New("index.template").Parse(embedded_index_template)
	panicOn(err)

	//vv("cfg = '%#v'", cfg)
	err = tmpl.Execute(&readyIndexHtmlBuf, cfg)
	panicOn(err)

	//vv("readyIndexHtmlBuf = '%v'\n", readyIndexHtmlBuf.String())

	pngs, err := filepath.Glob("*.png")
	panicOn(err)
	if len(pngs) == 0 {
		//fmt.Fprintf(os.Stderr, "no png files present.\n")
		//os.Exit(1)
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

	// have we saved to keepers already?
	var savedMut sync.Mutex
	saved := make(map[string]bool)

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
			saved[pngs[i]] = false
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

			alreadySaved := ""
			savedMut.Lock()
			if saved[curpng] {
				alreadySaved = ` <bold>saved to keepers</bold> <img class='left' src='/greencheckmark'>`
			}
			savedMut.Unlock()

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

			fmt.Fprintf(w, `<html>
<head>
<style type="text/css">
  .left{float:left;}
</style>
<script type = "text/JavaScript">`)

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
				// Up pressed: save to keepers
                document.getElementById("saved_to_keepers").innerHTML = " <bold>saved to keepers</bold> <img class='left' src='/keep/%s'>";
				break;
			case "ArrowDown":
				// Down pressed
				break;
			}
		}`, prevpng, nextpng, curpng)

			fmt.Fprintf(w, "%v</script></head><body>", script)
			fmt.Fprintf(w, `<font size="20">&nbsp;&nbsp;&nbsp;<a href="/view/%s">PREV</a>&nbsp;&nbsp;&nbsp;&nbsp;<a href="/view/%s">NEXT</a></font>&nbsp;[%03d&nbsp;of&nbsp;%03d]:&nbsp;%s &nbsp;&nbsp;&nbsp;<span id="saved_to_keepers"> %v </span><br>`, prevpng, nextpng, loc+1, n, curpng, alreadySaved)
			fmt.Fprintf(w, `<a href="/view/%s"><img src="/images/%s"></a><br>`, nextpng, curpng)
			fmt.Fprintf(w, `</body></html>`)
		}
		http.HandleFunc("/view/", viewHandler)
	}

	http.HandleFunc("/keep/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			vv("only GET supported")
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}
		path := r.URL.Path
		if containsDotDot(path) {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}
		keep := path[len("/keep/"):]
		//vv("request to keep = '%v'", keep)

		savedMut.Lock()
		_, ok := saved[keep]
		if ok {
			saved[keep] = true
		}
		savedMut.Unlock()
		if !ok {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}

		if !DirExists("keepers") {
			panicOn(os.MkdirAll("keepers", 0777))
		}
		keepby, err := ioutil.ReadFile(keep)
		panicOn(err)
		out, err := os.Create("keepers/" + keep)
		panicOn(err)
		nw, err := out.Write(keepby)
		panicOn(err)
		if nw != len(keepby) {
			panic(fmt.Sprintf("short write %v of %v on copy to keepers '%v'", nw, len(keepby), keep))
		}
		out.Close()

		w.Header().Set("Content-Type", "image/png")
		readSeeker := bytes.NewReader(savedToKeepersPng)
		modtime := time.Time{}
		http.ServeContent(w, r, "", modtime, readSeeker)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//http.ServeFile(w, r, "index.html")
		w.Write(readyIndexHtmlBuf.Bytes())
	})

	http.HandleFunc("/greencheckmark", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		readSeeker := bytes.NewReader(savedToKeepersPng)
		http.ServeContent(w, r, "", time.Time{}, readSeeker)
	})

	// So we have a portable archive that doesn't depend on copying
	// the directory of images, this is the default now:
	// Read from memory (equivalent to what is in the cfg.RbookFilePath / my.rbook file)
	http.HandleFunc("/rbook/", func(w http.ResponseWriter, r *http.Request) {

		if r.Method != "GET" {
			vv("only GET supported")
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}

		path := r.URL.Path

		if containsDotDot(path) {
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}

		path = path[len("/rbook"):]

		//vv("looking up path = '%v'", path)

		b.mut.Lock()
		defer b.mut.Unlock()

		e, ok := b.path2image[path]
		if !ok {
			vv("path '%v' not found in book path2image; path2image = '%#v'", path, b.path2image)
			http.Error(w, "invalid URL path", http.StatusBadRequest)
			return
		}
		//vv("path '%v' found in book path2image", path)
		w.Header().Set("Content-Type", "image/png")
		readSeeker := bytes.NewReader(e.ImageBy)
		modtime := e.Tm
		http.ServeContent(w, r, "", modtime, readSeeker)
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

func containsDotDot(v string) bool {
	if !strings.Contains(v, "..") {
		return false
	}
	for _, ent := range strings.FieldsFunc(v, isSlashRune) {
		if ent == ".." {
			return true
		}
	}
	return false
}

func isSlashRune(r rune) bool { return r == '/' || r == '\\' }

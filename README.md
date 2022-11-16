![rbooklogo](https://github.com/glycerine/rbook/blob/master/logo_rbook.png)

rbook
========

Project `rbook` provides an R notebook. `rbook` is 
written in Go, and a little C, for use with R. It works well
with emacs and ESS.

An rbook is similar to a Jupyter/ipython notebook, but 
is not built on them.

However, rbook has some of the same goals:

* show code and then graphs together on a web page

* save the sequence of code and graphs to disk for archive and review.

design
------

* goal: 

We want to enable saving and recording of an R session, including
plots, commands, and command output to the rbook. The
rbook is displayed in a web browser and updated
as the user's R session progresses. Should be usable
under an ESS/emacs environment.

* approach: 

To capture graphs, we run under Xvfb and
use the R savePlot() call. This only
happens on user demand. The user
calls sv() at the R prompt to save the 
current graph to the browser.

Interactive graph development is followed
in by running a vnc client attached to Xvfb session.
There are even web-based vnc clients like
https://guacamole.apache.org/ is one wishes; but
it appears to need a Java support proxy.

See also https://9to5answer.com/web-based-vnc-client
for non-Java pure websocket options.
They suggest https://github.com/InstantWebP2P/peer-vnc,
and mention PocketVNC but do not provide a link. Also,
mentioned are https://tightVNC.com, and
https://github.com/InstantWebP2P/peer-vnc .

These are minor things; running a native vnc client is simple
and effective. https://www.realvnc.com/en/ is free,
as are multiple alternatives.

            
* approach to show history in the browser:

All top level commands are captured by using
R's addTaskCallback mechanism. Our C
code then deparses each command, passes
it to Go, and Go conveys it to the
listening browser over a websocket.
            
* approach to showing command output (prints, etc):

To capture the output of commands, we use
the R sink() facility. Like graphs, printed command
output is only written to the browser on demand. The
user types dv() to "display the last value" in
the browser. We use the R sink() facility
to capture the last value seen at the R
top level.

* Push communication with listening browsers:

Rbook runs a web server to serve images to the browsers.
Simultanieously, rbook also runs a websocket interface 
that it uses to push to subscribed web browsers 
each new code/plot addition for display.

finished sub-tasks
------------------

[x] done. make history (the notebook) persistent so that
   browsers can reload history; even after R or
   the browser has been restarted.

[x]. have browser get the BookID and CreateTm from the server.
[x]. If the browser's log already has length:
      for new bookID, discard old log.
      for same bookID, just extend from log.

[x] done: it was simpler to have browser always discard all and then to
read all the history instead of trying to coordinate the log
number that the browser knows against the log / bookID current.


Still TODO
----------


_ pick the websocket port dynamically, embed into index.html before sending.
 to avoid collisions with multiples running at once.
 
_ we *could* auto-replay to re-obtain state as well... but might rather
step through it. But could be a nice option.

2) Automate the startup of the Xvfb, the window manager, and
   x11vnc server. Rbook should start them if they are
   not already running.

3) authentiation: deferred.

The login.go contains a simple cookie based login example
that would need to be made persistent to disk with
greenpack or other means. But we'll defer login until needed.

install notes
=============

How to get ESS to run rbook instead of default R:

~~~
(defun rbook ()
  (interactive)
  (let ((inferior-R-program-name "~/go/bin/rbook"))
  (R)))
~~~


howto - notes on figuring out what worked.
=====

~~~
xvfb-run R
~~~

we can be sure that there is always a local X environment
to write to. This can also be viewed in realtime with

~~~
Xvfb :99 -screen 0 3000x2000x16
icewm &
feh --bg-scale ~/pexels-ian-turnell-709552.jpg
x11vnc -display :99 -forever -nopw -quiet -xkb
~~~

Now a vnc client connecting to port 5900 will
show the xvfb frame buffer.

earlier notes; may be out of date
---------------------------------

In R
~~~
savePlot() # writes current plot to Rplot.png or filename=
~~~

We can invoke savePlot() to assign a filename,
and then send the filename to rbook.

Our rbook may then wish to copy the file
version of plots for safe keeping into the archive.
Update: yes, it copies them into a directory, .rbook,
in the current directory.

Since sometimes plots are built up interactively, we
may want to have a final() command added to R to tell
rbook to consolidate into just the nice finished plot.

how to get the code snippets
----------------------------

We'll try embedding R in our Go program 'rbook',
and have it all in one process, so that we get a chance to
see each command that comes through before
it is passed to R. 

This seems vastly better than hacking ESS and
trying to hook the code evaluation from there.

Since rbook should be saving everything to disk,
so if we have to restart that shouldn't be
a problem. Plus we'll know that rbook is
always up if the wrapped R is running. And
it is simple that it is all in the place; and
we cannot miss any commands that way.

Also with wrapping R, the wrapper Go code
can recognize plot commands automatically,
and not require anything extra complicated to
always save them to disk; perhaps copying
them into the archive, and associating them
with the code.

I'm liking this wrapper idea, simple single
process idea.

We have embedr already working and it provides
an API into executing arbitrary R code from Go.

~~~
import (
   "github.com/glycerine/embedr"
)

embedr.InitR()
defer embedr.EndR()
embedr.EvalR("R code here")

~~~

The R_ReplDLLinit() and embedr.ReplDLLdo1() was the key to 
getting a nice REPL experience under the R loaded as DLL.

https://cran.r-project.org/doc/manuals/R-exts.html#index-Rf_005finitEmbeddedR

https://rstudio.github.io/r-manuals/r-exts/Linking-GUIs-and-other-front-ends-to-R.html



notes 
-----
there is `max.deparse.length` as a limit, as an option to `source()` and a way to raise it

https://stackoverflow.com/questions/54872060/what-does-truncated-mean-in-the-tinn-r-console/55292384#55292384

https://emacs.stackexchange.com/questions/69220/ess-turn-off-truncated-in-ess-r-session

quoting a comment there:
~~~
It looks like I want to set max.depare.lengt = echo() rather than an integer. The relevant code seems to be in: ess/etc/ESSR/R/.basic.R: Specifically, 

.ess.eval <- function(string, visibly = TRUE, output = FALSE,              
                      max.deparse.length = 300,
                      file = tempfile("ESS"), local = NULL) { ...

and 

.ess.source <- function(file, visibly = TRUE, output = FALSE,
                        max.deparse.length = 300, local = NULL,
                        fake.source = FALSE, keep.source = TRUE,
                        message.prefix = "") { ...

mikemtnbikes
2022 Oct 25 
~~~

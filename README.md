minibook
========

A miniature or mini-version of a notebook server, 
written in Go, for use with R.

A minibook is similar to a Jupyter/ipython notebook, but 
is not built on them.

However, minibook has some of the same goals:

* show code and then graphs together on a web page

* save the sequence of code and graphs to disk for archive and review.

design
------

* goal: 

We want to enable saving and recording of an R session, including
plots, commands, and command output to the minibook. The
minibook is displayed in a web browser and updated
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

Mini runs a web server to serve images to the browsers.
Simultanieously, mini also runs a websocket interface 
that it uses to push to subscribed web browsers 
each new code/plot addition for display.

Still TODO
----------

1) make history (the notebook) persistent so that
   browsers can reload history; even after R or
   the browser has been restarted.
   
2) Automate the startup of the Xvfb, the window manager, and
   x11vnc server. Mini should start them if they are
   not already running.

3) authentiation: deferred.

The login.go contains a simple cookie based login example
that would need to be made persistent to disk with
greenpack or other means. But we'll defer login until needed.


howto
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
and then send the filename to minibook.

Our minibook may then wish to copy the file
version of plots for safe keeping into the archive.

Since sometimes plots are built up interactively, we
may want to have a final() command added to R to tell
mini to consolidate into just the nice finished plot.

how to get the code snippets
----------------------------

We'll try embedding R in our Go program 'mini',
and have it all in one process, so that we get a chance to
see each command that comes through before
it is passed to R. 

This seems vastly better than hacking ESS and
trying to hook the code evaluation from there.

Since mini should be saving everything to disk,
so if we have to restart that shouldn't be
a problem. Plus we'll know that mini is
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




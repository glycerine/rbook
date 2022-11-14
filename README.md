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

* goal: enable saving and pushing R plots to the minibook for
  display

* approach: mini embeds R to capture commands and notice
   when plots are made so they can be saved to disk, saved in the archive,
   and their paths sent to the web client.
   Simultanieously, mini also runs a websocket interface 
   that it uses to push to subscribed web browsers 
   each new code/plot addition for display.
   
The websocket code for browser clients already in the repo
from the golang-reload-browser example.

The login.go contains a simple cookie based login example
that would need to be made persistent to disk with
greenpack or other means. But we'll defer login until needed.

Capturing graphs in R and sending them to minibook:

By starting R under Xvfb using

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

We have embedr already working.

~~~
import (
   "github.com/glycerine/embedr"
)

embedr.InitR()
defer embedr.EndR()
embedr.EvalR("R code here")

~~~


rethink
-------

After some experience, the embedded R from the .so does not
give us the full R command line experience; prints and
warnings are missing.

Instead we just need to send saved-plot paths and commands
to the miniserver via rmq websocket.

Can elisp websockets go directly to a new minibook websocket
server API designed for it: in order to tell minibook
about new plot file paths, and new commands executed.

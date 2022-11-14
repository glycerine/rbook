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

* approach: minibook runs one websocket interface for R code to
   push code and graphics to. Along side it, minibook also runs a second
   websocket interface so that it uses to push to subscribed web browsers 
   each new code/plot addition.
   
The websocket code for pushing code/graphics from R to a 
websocket is already written in rmq in Go. It just
needs to be brought into minibook from rmq.

The rmq server should return the path to the graph on disk,
so that when we save our R session text file, it can refer
to those graphs, and perhaps even view them again without
having to regenerate them.

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

We can wrap savePlot() to assign a filename,
and then send the filename to minibook.

Our minibook may then wish to copy the file
version for safe keeping into the archive.

how to get the code snippets
----------------------------

The simplest thing to do would be to hook
the ess ctrl-n so that upon execution
of a line of R code, emacs also (somehow)
sends the line to the minibook server.

Or we could embed R in Go and have it all in
one process, so that we get a chance to
see each command that comes through before
it is passed to R. But we'd prefer that 
minibook be a separate process so that if
R crashes it stays serving web clients.

But still the wrapping of R might help in one
process, an "upgraded" R that also logs; and
then still talk to another minibook server?

But minibook should be saving everything to disk,
so if we have to restart that shouldn't be
a problem. Plus we'll know that miniserver is
always up if the wrapped R is running. And
it is simple that it is all in the place; and
we cannot miss any commands that way.

Still noodling on this design. 

Having to hack ess/emacs elisp code is not
much fun; kind of brittle.

Also with wrapping R, the wrapper Go code
can recognize plot commands automatically,
and not require anything extra to 
save them to disk; and log them.

I'm liking this wrapper idea, simple single
process idea, more.

We have embedr already working.

~~~
import (
   "github.com/glycerine/embedr" // theoretically; not there yet.
)

embedr.InitR()
defer embedr.EndR()
embedr.EvalR("R code here")

~~~




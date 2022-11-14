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



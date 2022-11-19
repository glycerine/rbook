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
There are even web-based vnc clients, but
that seems like extra work when 
running a native vnc client is simple
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

* Comments from the prompt into the book

Comments are created by having R evaluate a string literal
that starts with the hash symbol `#`.

For example, at the rbook prompt (the + signs are automatically 
added by the R REPL after the user presses enter in the
middle of a string literal, to indicate that a multi-line string
is being typed):

~~~
> "# start a comment line
+ that can span multiple lines
+ and is finished by ending the string literal"
~~~

is then rendered in the browser with a beige background
and `###` in front of each line.


The example above could equally have been done
with single quotes, since those also delimit string
literals in R. This may be easier to type, since
it does not involve the shift key typically.

~~~
'# start a comment line
+ that can span multiple lines
+ and is finished by ending the string literal'
~~~

In either case, the output is the same:

~~~
### start a comment line
### that can span multiple lines
### and is finished by the ending the string literal
~~~

R's evaluation engine simply ignores string literals
at the command prompt. They are legal values, but
are not assigned to anything and so change no state.
The rbook callbacks notice these special string
literals and display them nicely as left-aligned
`###` blocks, as is the convention for comments
in some places such as ESS.

From the ESS manual [1][2]:

> Comments are also handled specially by ESS, using an idea borrowed from the Emacs-Lisp indentation style. By default, comments beginning with ‘###’ are aligned to the beginning of the line. Comments beginning with ‘##’ are aligned to the current level of indentation for the block containing the comment. Finally, comments beginning with ‘#’ are aligned to a column on the right (the 40th column by default, but this value is controlled by the variable comment-column,) or just after the expression on the line containing the comment if it extends beyond the indentation column. You turn off the default behavior by adding the line (setq ess-indent-with-fancy-comments nil) to your .emacs file.


references

[1] http://ess.r-project.org/Manual/ess.html#Indenting

[2] https://stackoverflow.com/questions/780796/emacs-ess-mode-tabbing-for-comment-region



finished sub-tasks
------------------

[x] done. make history (the notebook) persistent so that
   browsers can reload history; even after R or
   the browser has been restarted.

[x]. have browser get the BookID and CreateTm from the server.

[x] done: keeping the browser state in sync 

it was simpler to have browser always discard all and then to
read all the history instead of trying to coordinate the log
number that the browser knows against the log / bookID current.

rbook sends the browser an init message now, so it knows
to discard the previous log of commands.

[x] mechanism to add comments into the stream.


Still TODO
----------


_ pick the websocket port dynamically, embed into index.html before sending.
 to avoid collisions with multiples running at once.

_ add configuration command line options for setting options/ the name
of the rbook file to save into.

_ maybe a command to read through the .rbook file and write out just
the R commands for easy text archive of the session.


_ we *could* auto-replay to re-obtain state as well... but might rather
step through it. But could be a nice option.

2) Automate the startup of the Xvfb, the window manager, and
   x11vnc server. Rbook should start them if they are
   not already running.

3) authentiation: deferred.

The login.go contains a simple cookie based login example
that would need to be made persistent to disk with
greenpack or other means. But we'll defer login until needed.

installation
=============

~~~
apt install Xvfb x11vnc icewm
~~~


How to get ESS to run rbook instead of default R:

~~~
(defun rbook ()
  (interactive)
  (let ((inferior-R-program-name "~/go/bin/rbook"))
  (R)))
~~~

To set `inferior-R-program-name` manually:

Ctrl-h v inferior-R-program-name -> position cursor over the customize link and press enter.

The emacs variable setting screen is shown:
~~~
inferior-R-program-name is a variable defined in ‘ess-custom.el’.
Its value is "/home/jaten/go/bin/rbook"
Original value was "R"

Documentation:
Program name for invoking an inferior ESS with M-x R.

You can customize this variable.
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

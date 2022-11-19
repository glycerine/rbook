![rbooklogo](https://github.com/glycerine/rbook/blob/master/logo_rbook.png)

rbook
========

Project `rbook` provides R notebooks,
affectionately known as rbooks.

The server binary itself is called simply `rbook`
and is used as a drop-in replacement
for `R` inside emacs.

When in an interactive R session, the rbook 
binary is also serving a live web view of the
session to any (or multple) web browsers. 

As plots are saved they are shown inline in realtime
with the R code, in the browser, as in a Jupyter notebook.
R output and free-form comments can also be logged.
Since all graphics, comments, code, and chosen output
are logged, rbooks form a simple, compact, and append-only
digital lab notebook for R.


detail
------

`rbook` is written in Go for use with R. It is designed for use
with emacs and ESS.


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

https://www.realvnc.com/en/ is a free VNC viewer.
There are multiple free alternatives.

            
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
user types dv() to "display the last console value" in
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

For example, at the rbook prompt[1]:

~~~
> "# start a comment line
+ that can span multiple lines
+ and is finished by ending the string literal"
~~~

is then rendered in the browser with a beige background
and `###` in front of each line.

[1] In the above example, the user does not type the '+' signs.
They are automatically added by the R REPL after the user 
presses enter in the middle of a string literal, to indicate that a multi-line string
is being typed.

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

From the ESS manual [2][3]:

> Comments are also handled specially by ESS, using an idea borrowed from the Emacs-Lisp indentation style. By default, comments beginning with ‘###’ are aligned to the beginning of the line. Comments beginning with ‘##’ are aligned to the current level of indentation for the block containing the comment. Finally, comments beginning with ‘#’ are aligned to a column on the right (the 40th column by default, but this value is controlled by the variable comment-column,) or just after the expression on the line containing the comment if it extends beyond the indentation column. You turn off the default behavior by adding the line (setq ess-indent-with-fancy-comments nil) to your .emacs file.


references

[2] http://ess.r-project.org/Manual/ess.html#Indenting

[3] https://stackoverflow.com/questions/780796/emacs-ess-mode-tabbing-for-comment-region



finished sub-tasks
------------------

[x] done. make history (the notebook) persistent so that
   browsers can reload history; even after R or
   the browser has been restarted.

[x]. have browser get the BookID and CreateTm from the server.

[x] done: keeping the browser state in sync 

[x] mechanism to add comments into the stream.

[x] done. pick the websocket port dynamically, embed into index.html before sending.
 to avoid collisions with multiples running at once.

[x] done. add configuration command line options for setting options/ the name
of the rbook file to save into.

[x] done. a parallel script/text version of the session is also written
for easy/quick review; without needing to open the browser. And if you
have only the binary rbook -dump will regenerate the text form.

[x] done: Automate the startup of the Xvfb, the window manager, and
   x11vnc server. Rbook should start them if they are
   not already running.

[ ] authentiation: deferred. None at the moment.

The login.go contains a simple cookie based login example
that would need to be made persistent to disk with
greenpack or other means. But we'll defer login until needed.

installation
=============

Preparation:
~~~
apt install Xvfb x11vnc icewm
~~~

This installs the dependencies.


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

earlier notes
-------------

In R
~~~
savePlot() # writes current plot to Rplot.png or filename=
~~~

We can invoke savePlot() to assign a filename,
and then send the filename to rbook.

Our rbook may then wish to copy the file
version of plots for safe keeping into the archive.
Update: yes, it copies them into a directory, my.rbook.plots,
in the current directory (assuming my.rbook is the
file name).

Since sometimes plots are built up interactively, we
wait until ready and given the final sv() command added to R to tell
rbook to consolidate into just the nice finished plot.

how to get the code snippets
----------------------------

We embeded R in our Go program 'rbook'.
So R and Go are all in one process. This

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

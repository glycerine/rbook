package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	"fmt"
)

var embedded_index_template = fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">

<head>
  <title>rbook R session</title>

  <!-- embedding the favacon.ico was the only reliable way to get it loaded.  -->
  %v
  
  <!--
  <link rel="icon" type="image/png" sizes="32x32" href="favicon-32x32.png">
  <link rel="icon" type="image/png" sizes="16x16" href="favicon-16x16.png">
  -->
  
  <!-- <link rel="stylesheet" href="//cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.6.0/build/styles/default.min.css"> -->

  <!-- how to choose a specific style: -->
  <link rel="stylesheet" href="/js_css/cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.6.0/build/styles/devibeans.min.css">

  <style>
    body {
        font-family:Consolas,Monaco,Lucida Console,Liberation Mono,DejaVu Sans Mono,Bitstream Vera Sans Mono,Courier New;
        font-weight: bold;
        font-size: 20px;

        /* dark background */
        background-color: #101010;
        /* light/white text color: */
        color: #ffffff;
    }

    /* https://stackoverflow.com/questions/16240684/css-code-highlighter-margin-in-pre-code-tag */
    pre > code { white-space: pre;
                 margin-top:  -0.50em;
                 display: block;
    }

/*  .RconsoleOutput {background-color: #e6e5e8; } */

    .RconsoleOutput {background-color: #792374; } /* purple-ish*/
    .RconsoleLine   {text-indent: 50px; }
    .Rcomment       {background-color: #7d8145;
                     /* background-color: #edf1b5; */
                     margin-top: 0.50em;
                     display: block;
                    }
    .Rcommand       {margin-top: -1.0em;
                     display: block;
                    }
    .RsecondCommandLine { color: rgba(0,0,0,0.4);
                        };

    .hidingOutputGrayout {
         // never seems to get applied to leave it out.
       // color: skyblue;
       // border: 2px solid red;
    }

    /*.RcommandLine   { margin-top: -0.1em; }*/

    </style>

   <script src="/js_css/cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.6.0/build/highlight.min.js"></script>

    <script type="text/javascript">

      var globalLastSeqno = -1;
      var lineNum = 1;

      function noNumbers(e) {
          this.value = this.value.replace(/[^\d]/, '');
      }

      // down arrow scrolls to bottom of page, otherwise leave current view unchanged.
      function checkKey(event) {
            //console.log("event: ", event);
            if (event.shiftKey) {
               switch (event.key) {
                  case "ArrowDown":
                     // shift + Down pressed
                     //scrollToLastID();
                     scrollToEndOfLog();
                     break;
                  case "ArrowUp":
                     // shift + Up pressed
                     window.scrollTo(0, 0);
                     break;
                }
             }
             if (event.key == "g") {
                // goto line numer dialog box mechanics
                var gotolineSubmitButton = document.getElementById("gotoDialogOK");
                gotoDialogOK.addEventListener('click', goToLine);

                var gotoDialogBox = document.getElementById("myGotoLineDialog");
                var gotoLineEntry = document.getElementById("line_request");
                // only accept numbers, thus tossing out the 'g' too.
                gotoLineEntry.value = '';
                gotoLineEntry.addEventListener('input', noNumbers, false);
                gotoDialogBox.showModal();
             }
      }
      document.onkeydown = checkKey;

      function goToLine() {
          let lineNum0 = document.querySelector('input').value;
          //console.log("goto diaglog sees input: ", lineNum0);
          if (lineNum0 != '') {
             var line0 = parseInt(lineNum0);
             if (line0 > lineNum) {
                 scrollToEndOfLog();
                 return;
             }
             var lineNumClass = 'line_' + lineNum0.toString();
             //console.log("trying to scroll to " + lineNumClass);

             var list = document.querySelectorAll('.'+lineNumClass);
             if (list.length > 0) {
                 d=list[0];
                 //console.log("found element to scroll to: ", d);
                 d.scrollIntoView({behavior:"instant", block: "start", inline: "nearest"}); 
             }
          }
      }

      function stamp() {
          var dt = new Date();
          //document.getElementById("datetime").innerHTML = dt.toLocaleString();
          document.getElementById("datetime").innerHTML = dt.toTimeString() + "   " + dt.toISOString();
      }
      
      function disableScroll() {
          // Get the current page scroll position
          scrollTop = window.pageYOffset || document.documentElement.scrollTop;
          scrollLeft = window.pageXOffset || document.documentElement.scrollLeft,

          // if any scroll is attempted, set this to the previous value
          window.onscroll = function() {
              window.scrollTo(scrollLeft, scrollTop);
          };
      }

      function enableScroll() {
          window.onscroll = function() {};
      }
      
/**
 * Tries to connect to the reload service and start listening to reload events.
 *
 * @function tryConnectToReload
 * @public
 */
function tryConnectToReload(address) {
  var conn = new WebSocket(address);

  conn.onclose = function() {
    globalLastSeqno = -1;
    setTimeout(function() {
      tryConnectToReload(address);
    }, 2000);
  };

    var partialMsg = "";
    conn.onmessage = function(evt) {
        // console.log("onmessage: evt.data = <<<<<", evt.data,">>>>>");

        // We send length prefixed messages, in case they get concatenated.
        // Break them up and append them separately here.


        var remain = "";
        if (partialMsg.length > 0) {
            remain = partialMsg + evt.data;
            partialMsg = "";
        } else {
            remain = evt.data;
        }

        // messages also seem to be newline delimitted, so use that.
        var lines = remain.split("\n");
        var i = 0;
        for (i = 0; i < lines.length; i++) {
           var line = lines[i]; 

           var colon = line.indexOf(":")
           if (colon <= 0) {
               partialMsg = lines.slice(i).join("\n");
               // wait for more on next onmessage() callback.
               return;
           }
           //var len = parseInt(line.substring(0, colon).trim());
           //var lastCurly = msg.lastIndexOf("}");
           var msg = line.substring(colon+1)
           appendLog(msg);    
        }
        // try just the newlines alone. very simple.
        /*
        var colon = line.indexOf(":")
        while (colon > 0) {
                var len = parseInt(line.substring(0, colon).trim());
                if (line.length - colon - 1 < len) {
                    console.log("incomplete message, not enough line.len=", line.length, " to satisfy len =", len);
                    break;
                }
                var msg = line.substring(colon+1, Math.min(colon+1+len, line.length));

                // len can be too long if the escaped characters then compress back,
                // so we'll also insist on ending with a curly brace + newline, which should
                // be the last thing in the textual JSON object.
                var lastCurly = msg.lastIndexOf("}\n") + 1;
                if (lastCurly <= 0) {
                     console.log("incomplete message, wait for more: '", msg, "'");
                     break;
                }

                if (len > lastCurly) {
                    console.log("problem: len was too long: len= ", len, " but lastCurly =  ", lastCurly, " msg='", msg, "'");
                    len = lastCurly;
                    msg = line.substring(colon+1, colon+1, len);
                }
                appendLog(msg);
                remain = remain.substring(colon+2+len);
                if (remain.length == 0) {
                    break;
                }
                colon = remain.indexOf(":");
           }
           partialMsg = remain;
           console.log("set partialMsg = '", partialMsg, "'");
           */
  };
}


function scrollToBottom() {
    window.scrollTo(0, document.body.scrollHeight);
}

function nextID() {
    var d  = document.getElementById("log");
    var n  = d.children.length;
    var id  = "log_" + n.toString();
    return id;
}

function lastID() {
    var d  = document.getElementById("log");
    var n  = d.children.length -1;
    var id  = "log_" + n.toString();
    return id;
}

function scrollToLastID() {
    var d = document.getElementById(lastID());
    if (d === null) { 
      // don't deference it.
    } else {
      // align the bottom of the element to the bottom of the viewport 
      d.scrollIntoView({behavior:"instant", block: "start", inline: "nearest"}); 
    }
}

function scrollToEndOfLog() {
    var d = document.getElementById("end-of-log");
    if (d === null) { 
      // don't deference it.
    } else {
      d.scrollIntoView(true); // align the bottom of the element to the bottom of the viewport 
    }
}


function nextIDInt() {
    var d  = document.getElementById("log");
    return d.children.length;
}

function pad(num, size) {
    num = num.toString();
    while (num.length < size) num = "0" + num;
    return num;
}

function rendered() {
    //Render complete
    //alert("image rendered");
    console.log("rendered() called. render complete");
    scrollToEndOfLog();
}

function startRender() {
    //Rendering start
    requestAnimationFrame(rendered);
}

function loaded()  {
    requestAnimationFrame(startRender);
}

function hideConsoleOutputDoubleClick(seqno) {
    var hideClass = 'seqno_class_' + seqno;
    var elements = document.getElementsByClassName(hideClass)
    for (var i = 0; i < elements.length; i++){
        elements[i].style.display = 'none';
    }
   var topLine = document.getElementsByClassName('seqno_firstline_class_'+seqno)[0]
   topLine.classList.add('hidingOutputGrayout');
   topLine.hiddenInnerHTML = topLine.innerHTML;
   topLine.innerHTML = '                ...     (console output hidden; double click to show)';
   // in case we were a long ways from the top, bring the top back into view
   if (elements.length > 40) {
      //topLine.scrollIntoView({behavior:"instant", block: "start", inline: "nearest"}); 
      topLine.parentNode.scrollIntoView(true);
   }
}

function showConsoleOutputDoubleClick(seqno) {
    var showClass = 'seqno_class_' + seqno;
    var elements = document.getElementsByClassName(showClass)
    for (var i = 0; i < elements.length; i++){
        elements[i].style.display = '';
    }
   var topLine = document.getElementsByClassName('seqno_firstline_class_'+seqno)[0]
   topLine.classList.remove('hidingOutputGrayout');
   if (topLine.hiddenInnerHTML) {
      topLine.innerHTML = topLine.hiddenInnerHTML;
   }
}


function toggleConsoleOutputDoubleClick(seqno) {
    var toggleClass = 'seqno_topparent_'+seqno;
    var elements = document.getElementsByClassName(toggleClass)
    if (elements) {
        if (elements.length == 0) {
           return;
        }
        var parent = elements[0];
        if (parent.isRbookCompressed) {
           parent.isRbookCompressed='';
           showConsoleOutputDoubleClick(seqno);
        } else {
           parent.isRbookCompressed='compressed';
           hideConsoleOutputDoubleClick(seqno);
        }
    }
}
      
function appendLog(msg){
 
    //console.log("msg = ", msg);
    
    const update = JSON.parse(msg)

    var d  = document.getElementById("log");

    if (update.comment) {
         //console.log("we just saw comment message: ", update.comment);
         var newstuff = '<div id="' + nextID() + '" class="Rcomment">';

        for (let i = 0; i < update.comment.length; i++) {
            newstuff += '<div class="RcommentLine">' + update.comment[i] + '</div>';
        }
         newstuff += '</div>';
         var newDiv = document.createElement('div');
         newDiv.innerHTML = newstuff;
         d.appendChild(newDiv);
         //d.innerHTML += newstuff + '</div>';
         //console.log("we added a comment block")
    }
     
    if (update.init) {
         console.log("we just saw init message: ", update.init);
         lineNum = 1;
         document.getElementById("bookID").innerHTML = '#' + update.book.user + "@" + update.book.host + ":" + update.book.path + "<br/>#BookID:" + update.book.bookID;
         document.getElementById("datetime").innerHTML = update.book.createTm;
         globalLastSeqno = -1;
         // this clears all previous log entries/cells.
         d.innerHTML = "";         
    }
     
    // try to prevent duplicates due to websocket tomfoolery.
     if (update.seqno) {
         // recognize a refresh from the start
         if (update.seqno == 0) {
             console.log("update.seqno is 0, clearing all and restarting: update=", update);
             globalLastSeqno = -1;
             // this clears all previous log entries/cells.
             d.innerHTML = "";
         }
         
        if (update.seqno > globalLastSeqno) {
            // good keep it
            globalLastSeqno = update.seqno;
        } else {
           // drop duplicates
           //console.log("dropping stale message update.seqno" + update.seqno + " vs. last " + globalLastSeqno);
           return;
        }
    }

    if (update.overlayHideSeqno) {
         // TODO
         // hide the update.overlayHideSeqno number element
    }
    if (update.overlayNote) {
         // TODO
         // overlay update.overlayNote onto update.overlayOnSeqno
    }

    if (update.command) {
         //console.log("we just saw command message: ", update.command);

         var newstuff = '<div id="' + nextID() + '" class="Rcommand"><pre><code>';

         for (let i = 0; i < update.command.length; i++) {
             var lineNumClass = 'line_' + lineNum.toString();
             var lineNumStr = '[' + pad(lineNum++,3) + ']';
             if (i > 0) {
                 lineNumStr = '<span class="RsecondCommandLine">' + lineNumStr + '</span>';
             }
             var cmdi = hljs.highlight(update.command[i], {language: 'R'}).value;

             newstuff += '<div class="RcommandLine '+lineNumClass+'">'  + lineNumStr + ' ' + cmdi + '</div>';
         }
         newstuff += '</code></pre></div>';
         var newDiv = document.createElement('div');
         newDiv.innerHTML = newstuff;
         d.appendChild(newDiv);
         //d.innerHTML += newstuff + '</code></pre></div>';
         //console.log("we added a command block")

        //var newstuff = '<div id="' + nextID() + '">' + update.command + '</div>';
        //d.innerHTML += newstuff
        //console.log("we added command text");
    }

    // in theory the command and the output could arrive together, so
    // print the console output after the text of the command.
    if (update.console) {
        //var hideDoubleClickFun = ' ondblclick="hideConsoleOutputDoubleClick(' + update.seqno + ')" ';
        //var showDoubleClickFun = ' ondblclick="showConsoleOutputDoubleClick(' + update.seqno + ')" ';
        var toggleDoubleClickFun = ' ondblclick="toggleConsoleOutputDoubleClick(' + update.seqno + ')" ';
        var hideDoubleClickFun = toggleDoubleClickFun;
        var showDoubleClickFun = toggleDoubleClickFun;

        var isLong = false;

        // use seqno_topparent_3319 class as an ID to locate the "compressed" or not state.
        var newstuff = '<div id="' + nextID() + '" class="RconsoleOutput seqno_topparent_'+update.seqno+'"><pre><code>';

         if (update.console.length >= 40) {
            // special case handling for very long output so we still show the top/bottom 15 lines
            isLong = true;

            var tailLoc = update.console.length - 15;
            for (let i = 0; i < update.console.length; i++) {
               if (i==15) {
                  // make the 15th (middle-ish?) line the one that changes content
                  newstuff += '<div class="RconsoleLine seqno_firstline_class_' + update.seqno + '" '+showDoubleClickFun+'>' + update.console[i] + '</div>';
               } else if (i > 15 && i < tailLoc) {
                  // we put seqno_class_3119 for example to compress the middle lines.
                  newstuff += '<div class="RconsoleLine seqno_class_' + update.seqno + '" '+hideDoubleClickFun+'>' + update.console[i] + '</div>';
               } else {
                  // head or tail lines:
                  // these get the double click function but NOT the seqno_class so they stay visible always.
                  newstuff += '<div class="RconsoleLine" '+hideDoubleClickFun+'>' + update.console[i] + '</div>';
               }
            }

         } else {
            // regular, short console output, <= 40 lines.

            for (let i = 0; i < update.console.length; i++) {
               if (i==0) {
                  newstuff += '<div class="RconsoleLine seqno_firstline_class_' + update.seqno + '" '+showDoubleClickFun+'>' + update.console[i] + '</div>';
               } else {
                  // we put seqno_class_3119 only on the 2nd and later lines that we will fold in
                  // according to any later overlay request to hide a big output from seqno 3119.
                  newstuff += '<div class="RconsoleLine seqno_class_' + update.seqno + '" '+hideDoubleClickFun+'>' + update.console[i] + '</div>';
               }
            }
         }
         newstuff += '</code></pre></div>';
         var newDiv = document.createElement('div');
         newDiv.innerHTML = newstuff;
         d.appendChild(newDiv);

         if (isLong) {
             // auto-hide long outputs until double-clicked to show.
             toggleConsoleOutputDoubleClick(update.seqno);
         }
        //console.log("we added console output");
    }

    if (update.image) {
        var hash = "";
        if (update.pathhash) {
           hash = update.pathhash;
        }
        // remove the leading slash(es) from update.image to avoid the 247msec network 301 redirect
        // that happens when seeing host:port/rbook//path -> host:port/rbook/path
        var upimg = update.image.replace(/^\/+/, '');
        //var urlhost = {{.WsHost}};
        var urlhost = window.location.hostname; // .host has the :port too, which we elide.

        var newstuff = '<div id="'+ nextID() +'" style="max-width: 800px"><img src="http://'+urlhost+':{{.Port}}/rbook/' + upimg + '?pathhash=' + hash + '" style="max-width:100%%;"/></div>';

         var newDiv = document.createElement('div');
         newDiv.innerHTML = newstuff;
         d.appendChild(newDiv);

        //d.innerHTML += newstuff;        
    }
    
    //hljs.highlightAll();

    // scroll to the bottom to show the latest output.
    // 
    // 2 msec isn't long enough to win the fight for the scrollbar
    // position, usually. but 20 msec seems to win it consistently.
    //
    //setTimeout(function() { /*console.log("called back!");*/ scrollToEndOfLog();}, 500);
    //requestIdleCallback(function(idleDeadline) { scrollToEndOfLog(); console.log("done with idle scroll");}, {timeout: 1000});

    //scrollToEndOfLog();
    //scrollToLastID();
    //scrollToBottom();

    // https://stackoverflow.com/questions/14578356/how-to-detect-when-an-image-has-finished-rendering-in-the-browser-i-e-painted
    //requestAnimationFrame(startRender);
    
} // end appendLog()
var urlhost = window.location.hostname;
try {
  if (window["WebSocket"]) {
    // The reload endpoint is hosted on a statically defined port.
    try {
      //tryConnectToReload("ws://{{.WsHost}}:{{.WsPort}}/reload");
      tryConnectToReload("ws://"+urlhost+":{{.WsPort}}/reload");
    }
    catch (ex) {
      // If an exception is thrown, that means that we couldn't connect to to WebSockets because of mixed content
      // security restrictions, so we try to connect using wss.
      //tryConnectToReload("wss://{{.WsHost}}:{{.WssPort}}/reload");
      tryConnectToReload("wss://"+urlhost+":{{.WssPort}}/reload");
    }
  } else {
    console.log("Your browser does not support WebSockets, cannot connect to the Reload service.");
  }
} catch (ex) {
  console.error('Exception during connecting to Reload:', ex);
}
</script>
</head>

<body>
  <dialog id="myGotoLineDialog">
      <form method="dialog">
          <label>goto line:<input name="line_req" id="line_request" placeholder="enter a line number"/></label>
          <button id="gotoDialogOK" value="default" hidden>ok</button>
      </form>
  </dialog>
  
  <p><span id="bookID"></span><br/>
    #R rbook created: <span id="datetime"></span></p>
[g: goto line || shift-down: end-of-log || shift-up: pop to top || shift-space: page up || space: page down]
  <p/>
  <br/>
  <div id="log"> </div>
  <div id="end-of-log">--- end of log --- [g: goto line || shift-down: end-of-log || shift-up: pop to top || shift-space: page up || space: page down]</div>
</body>

</html>
`, RHashFavIcon)

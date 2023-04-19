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
  <link rel="stylesheet" href="//cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.6.0/build/styles/devibeans.min.css">

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

    /*.RcommandLine   { margin-top: -0.1em; }*/

    </style>

   <script src="//cdn.jsdelivr.net/gh/highlightjs/cdn-release@11.6.0/build/highlight.min.js"></script>

    <script type="text/javascript">

      var globalLastSeqno = -1;

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

// causes Chrome to freeze/spin forever on 1k-2k lines of rbook.
function scrollToLastID() {
    var d = document.getElementById(lastID());
    if (d === null) { 
      // don't deference it.
    } else {
      d.scrollIntoView(true); 
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

var lineNum = 1;
      
function appendLog(msg){
 
    console.log("msg = ", msg);
    
    const update = JSON.parse(msg)

    var d  = document.getElementById("log");

    if (update.comment) {
         //console.log("we just saw comment message: ", update.comment);
         var newstuff = '<div id="' + nextID() + '" class="Rcomment">';

        for (let i = 0; i < update.comment.length; i++) {
            newstuff += '<div class="RcommentLine">' + update.comment[i] + '</div>';
        }
         d.innerHTML += newstuff + '</div>';         
         //console.log("we added a comment block")
    }
     
    if (update.init) {
         //console.log("we just saw init message: ", update.init);
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

    if (update.command) {
         //console.log("we just saw command message: ", update.command);

         var newstuff = '<div id="' + nextID() + '" class="Rcommand"><pre><code>';

         for (let i = 0; i < update.command.length; i++) {
             var lineNumStr = '[' + pad(lineNum++,3) + ']';
             if (i > 0) {
                 lineNumStr = '<span class="RsecondCommandLine">' + lineNumStr + '</span>';
             }
             var cmdi = hljs.highlight(update.command[i], {language: 'R'}).value;

             newstuff += '<div class="RcommandLine">'  + lineNumStr + ' ' + cmdi + '</div>';
         }
         d.innerHTML += newstuff + '</code></pre></div>';
         //console.log("we added a command block")

        //var newstuff = '<div id="' + nextID() + '">' + update.command + '</div>';
        //d.innerHTML += newstuff
        //console.log("we added command text");
    }

    // in theory the command and the output could arrive together, so
    // print the console output after the text of the command.
    if (update.console) {
        var newstuff = '<div id="' + nextID() + '" class="RconsoleOutput"><pre><code>';
        for (let i = 0; i < update.console.length; i++) {
            newstuff += '<div class="RconsoleLine">' + update.console[i] + '</div>';
        }
        d.innerHTML += newstuff + '</code></pre></div>';
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

        var newstuff = '<div id="'+ nextID() +'" style="max-width: 800px"><img src="http://{{.WsHost}}:{{.Port}}/rbook/' + upimg + '?pathhash=' + hash + '" style="max-width:100%%;"/></div>';
        d.innerHTML += newstuff;        
    }
    
    //hljs.highlightAll();

    // scroll to the bottom to show the latest output.
    // 
    // 2 msec isn't long enough to win the fight for the scrollbar
    // position, usually. but 20 msec seems to win it consistently.
    //
    //setTimeout(function() { /*console.log("called back!");*/ scrollToBottom()}, 20);

    // this causes Chrome to spin forever on a couple thousand log lines.
    // We'll omit it and hope to make the 15 minute pauses go away.
    //scrollToLastID()

    
} // end appendLog()

try {
  if (window["WebSocket"]) {
    // The reload endpoint is hosted on a statically defined port.
    try {
      tryConnectToReload("ws://{{.WsHost}}:{{.WsPort}}/reload");
    }
    catch (ex) {
      // If an exception is thrown, that means that we couldn't connect to to WebSockets because of mixed content
      // security restrictions, so we try to connect using wss.
      tryConnectToReload("wss://{{.WsHost}}:{{.WssPort}}/reload");
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
  <p><span id="bookID"></span><br/>
    #R rbook created: <span id="datetime"></span></p>
  <br/>
  <div id="log"> </div>
</body>

</html>
`, RHashFavIcon)

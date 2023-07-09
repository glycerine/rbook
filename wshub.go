package main

// Copyright (C) 2022 Jason E. Aten, Ph.D. All rights reserved.

import (
	"fmt"
	"time"

	"github.com/glycerine/embedr"
)

var _ = fmt.Printf
var _ = embedr.SetCustomPrompt

// portions of this code re-used under this license:
/*
MIT License

Copyright (c) 2016 Mark Vincze

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *HashRElem

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	book *HashRBook
}

func newHub(book *HashRBook) *Hub {
	return &Hub{
		book:       book,
		broadcast:  make(chan *HashRElem),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
top:
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			cc := client.conn
			_ = cc
			// conn embeds net.Conn
			ncli := len(h.clients)
			_ = ncli
			vvlog("websocket client (count %v) remote:%v", ncli, cc.RemoteAddr().String())
			//embedr.SetCustomPrompt(fmt.Sprintf("[wsclient: %v] >", ncli))

			// give the new client all the book, starting with the init message
			h.book.mut.Lock()
			select {
			case client.send <- []byte(prepInitMessage(h.book)):
				vvlog("sent init msg to new client, have %v updates to follow", len(h.book.elems))
			case <-time.After(300 * time.Second):
				//default:
				vvlog("client.send could not proceed after 300 seconds.")
				close(client.send)
				delete(h.clients, client)
				h.book.mut.Unlock()
				continue top // don't try to send below on closed channel!
			}
			for i, e := range h.book.elems {
				_ = i
				//vv("updating new client with seqno %v", e.Seqno)
				select {
				//huh. got after long inactivity, panic: send on closed channel
				// also got when old client coming back to new server.
				case client.send <- e.msg:
				case <-time.After(300 * time.Second):
					//default:
					// this was the closing too early culprit!
					//vv("closing client.send after i=%v out of %v", i, len(h.book.elems))
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.book.mut.Unlock()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				//vv("closed client.send after unregister")
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message.msg:
				case <-time.After(10 * time.Second):
					//default:
					close(client.send)
					delete(h.clients, client)
					//vv("closed client.send after broadcast failure")
				}
			}
		}
	}
}

package main

import (
	"fmt"
	"github.com/glycerine/embedr"
)

var _ = fmt.Printf
var _ = embedr.SetCustomPrompt

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
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			cc := client.conn
			_ = cc
			// conn embeds net.Conn
			ncli := len(h.clients)
			_ = ncli
			//vv("websocket client (count %v) remote:%v", ncli, cc.RemoteAddr().String())
			//embedr.SetCustomPrompt(fmt.Sprintf("[wsclient: %v] >", ncli))

			// give the new client all the book, starting with the init message
			h.book.mut.Lock()
			select {
			case client.send <- []byte(prepInitMessage(h.book)):
				vv("sent init msg to new client, have %v updates to follow", len(h.book.Elems))
			default:
				close(client.send)
				delete(h.clients, client)
			}
			for _, e := range h.book.Elems {
				vv("updating new client with seqno %v", e.Seqno)
				select {
				case client.send <- e.msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.book.mut.Unlock()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message.msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

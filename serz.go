package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/glycerine/cryrand"
	//snappy "github.com/glycerine/go-unsnap-stream"
	"github.com/glycerine/greenpack/msgp"
)

//go:generate greenpack

// HashRTyp tells us the type of a HashRElem.
type HashRTyp int

const (
	Command HashRTyp = 1
	Console HashRTyp = 2
	Image   HashRTyp = 4
	Comment HashRTyp = 8
)

func (ty HashRTyp) String() string {
	switch ty {
	case Command:
		return "Command"
	case Comment:
		return "Comment"
	case Console:
		return "Console"
	case Image:
		return "Image"
	}
	panic(fmt.Sprintf("unrecognized HashRTyp = %v", int(ty)))
}

// HashRElem are the "cells" of a notebook; in this
// case the elements of the HashRBook.Elems
type HashRElem struct {

	// Typ tells us which of the 3 types of content do we have.
	//
	// Althought the Typ bits are setup to allow
	// any mixture, we start with exact one type
	// in each Elem to unambiguously preserve the sequence order
	// in which they occur, so that replay is idempotent.
	//
	Typ HashRTyp `msg:"type" json:"type" zid:"0"`

	// timestamp when written.
	Tm time.Time `msg:"tm" json:"tm" zid:"1"`

	Seqno int `msg:"seqno" json:"seqno" zid:"2"`

	// These JSON strings are exactly what we shipped to the
	// browser the first time. So they are ready
	// for replay.

	// 1st and most common type: top level R commands
	CmdJSON string `msg:"cmdJSON" json:"cmdJSON" zid:"3"`

	// 2nd: console output from running R commands.
	ConsoleJSON string `msg:"consoleJSON" json:"consoleJSON" zid:"4"`

	// 3rd type: image
	ImageJSON string `msg:"imageJSON" json:"imageJSON" zid:"5"`

	// 4th type: comment
	CommentJSON string `msg:"commentJSON" json:"commentJSON" zid:"6"`

	// where it was on disk;
	ImageHost string `msg:"imageHost" json:"imageHost" zid:"7"`
	ImagePath string `msg:"imagePath" json:"imagePath" zid:"8"`

	// ImageBy has png formatted graphic, referred to by ImageJSON and ImagePath;
	// checksummed by ImagePathHash.
	ImageBy []byte `msg:"imageBy" json:"imageBy" zid:"9"`

	// ImagePathHash = hash(ImageHost + ImagePath + ImageBy)
	ImagePathHash string `msg:"imagePathHash" json:"imagePathHash" zid:"10"`

	// convenience, not on disk.
	msg []byte
}

func (e *HashRElem) String() (s string) {
	return fmt.Sprintf(`
HashRElem{
	Typ: %v,
	Tm: %v,
	Seqno: %v,
	CmdJSON: %v,
	ConsoleJSON: %v,
	CommentJSON: %v,
	ImageJSON: %v,
	ImageHost: %v,
	ImagePath: %v,
	ImageBy: (len: %v),
	ImagePathHash: %v,
}
`, e.Typ, e.Tm, e.Seqno, e.CmdJSON, e.ConsoleJSON, e.CommentJSON, e.ImageJSON, e.ImageHost, e.ImagePath, len(e.ImageBy), e.ImagePathHash)
}

// The header, aka init message.
type HashRBook struct {
	CreateTm time.Time `msg:"createTm" json:"createTm" zid:"0"`

	// unique ID for notebook; so we don't confuse clients
	// when switching notebooks; like UUID but not.
	BookID string `msg:"bookID" json:"bookID" zid:"1"`

	User string `msg:"user" json:"user" zid:"2"`
	Host string `msg:"host" json:"host" zid:"3"`
	Path string `msg:"path" json:"path" zid:"4"`

	// held for convenience here, but serialized
	// separately afterwards on disk and individually
	// on the wire, so lower case elems.
	elems []*HashRElem `msg:"elems" json:"elems" zid:"5"`

	// must hold when reading/writing elems
	mut sync.Mutex
}

func NewHashRBook(user, host, path string) *HashRBook {
	return &HashRBook{
		CreateTm: time.Now(),
		BookID:   cryrand.RandomStringWithUp(24),
		User:     user,
		Host:     host,
		Path:     path,
	}
}

func ReadBook(user, host, path string) (h *HashRBook, appendFD *os.File, err error) {

	fresh := true
	if FileExists(path) {
		sz, err := FileSize(path)
		panicOn(err)
		if sz != 0 {
			fresh = false
		}
	}
	appendFD, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0660)
	panicOn(err)

	if fresh {
		// nothing to read
		h = NewHashRBook(user, host, path)

		var by []byte
		by, err = h.SaveToSlice()
		panicOn(err)
		_, err = appendFD.Write(by)
		return
	}

	mpr := msgp.NewReader(appendFD)
	h, err = LoadBook(mpr)
	panicOn(err)

	var e *HashRElem
	for {
		e, err = LoadElem(mpr)
		if err == io.EOF {
			err = nil
			return
		}
		panicOn(err)
		//vv("got '%v'", e)
		h.elems = append(h.elems, e)
	}
	return
}

type ByteSlice []byte

// read a HashRElem into e from r.
func LoadElem(r *msgp.Reader) (e *HashRElem, err error) {

	// peek ahead first, so we can avoid
	// moving the read point ahead if there
	// are insufficient bytes

	// try to get at least 5 bytes, but
	// settle for 2 since that is possible.
	var by []byte

	var i int
	for i = 5; i >= 2; i-- {
		by, err = r.R.Peek(i)
		if err == nil {
			break
		}
		if err == io.EOF {
			// try shorter
			continue
		}
		return nil, err
	}
	if err == io.EOF {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error trying to r.R.Peek() for bytes: '%s'", err)
	}

	//ninside, ntotal, nheader, err := UnframeBinMsgpack(by)
	ntotal, _, _, err := UnframeBinMsgpack(by)

	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on UnframeBinMsgPack(): '%s'", err)
	}

	_, err = r.R.Peek(ntotal)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on Peek() call for ntotal bytes: '%s'", err)
	}

	var bs2 ByteSlice
	err = bs2.DecodeMsg(r)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on ByteSlice(by).DecodeMsg(): '%s'", err)
	}

	var ue HashRElem
	_, err = ue.UnmarshalMsg(bs2)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on tk.UnmarshalMsg(): '%s'", err)
	}

	// fill the msg convenience for refreshing new clients with history
	switch ue.Typ {
	case Command:
		ue.msg = []byte(ue.CmdJSON)
	case Console:
		ue.msg = []byte(ue.ConsoleJSON)
	case Image:
		ue.msg = []byte(ue.ImageJSON)
	case Comment:
		ue.msg = []byte(ue.CommentJSON)
	}

	return &ue, nil
}

// Save HashRElem as a framed msgpack message (where first few bytes are a []byte encoded
// to tell us the size of the rest of the bytes that follow. Those following
// bytes consist themselves of a msgpack serialized Tk.
func (e *HashRElem) SaveToSlice() ([]byte, error) {

	b, err := e.MarshalMsg(nil)
	if err != nil {
		return nil, fmt.Errorf("HashRElem.SaveToSlice() error on MarshalMsg: '%s'", err)
	}
	return ByteSlice(b).MarshalMsg(nil)
}

// read a HashRBook into book from r.
func LoadBook(r *msgp.Reader) (book *HashRBook, err error) {

	// peek ahead first, so we can avoid
	// moving the read point ahead if there
	// are insufficient bytes

	// try to get at least 5 bytes, but
	// settle for 2 since that is possible.
	var by []byte

	var i int
	for i = 5; i >= 2; i-- {
		by, err = r.R.Peek(i)
		if err == nil {
			break
		}
		if err == io.EOF {
			// try shorter
			continue
		}
		return nil, err
	}
	if err == io.EOF {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error trying to r.R.Peek() for bytes: '%s'", err)
	}

	//ninside, ntotal, nheader, err := UnframeBinMsgpack(by)
	ntotal, _, _, err := UnframeBinMsgpack(by)

	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on UnframeBinMsgPack(): '%s'", err)
	}

	_, err = r.R.Peek(ntotal)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on Peek() call for ntotal bytes: '%s'", err)
	}

	var bs2 ByteSlice
	err = bs2.DecodeMsg(r)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on ByteSlice(by).DecodeMsg(): '%s'", err)
	}

	var ue HashRBook
	_, err = ue.UnmarshalMsg(bs2)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on tk.UnmarshalMsg(): '%s'", err)
	}

	return &ue, nil
}

// Save HashRBook as a framed msgpack message (where first few bytes are a []byte encoded
// to tell us the size of the rest of the bytes that follow. Those following
// bytes consist themselves of a msgpack serialized Tk.
func (book *HashRBook) SaveToSlice() ([]byte, error) {

	b, err := book.MarshalMsg(nil)
	if err != nil {
		return nil, fmt.Errorf("HashRBook.SaveToSlice() error on MarshalMsg: '%s'", err)
	}
	return ByteSlice(b).MarshalMsg(nil)
}

const (
	bin8  uint8 = 0xc4
	bin16 uint8 = 0xc5
	bin32 uint8 = 0xc6
)

type UnframeError int

const (
	NotEnoughBytes UnframeError = -1
	NotBinarySlice UnframeError = -2
)

func (e UnframeError) Error() string {
	switch e {
	case NotEnoughBytes:
		return "UnframeBinMsgpack() error: NotEnoughBytes"
	case NotBinarySlice:
		return "UnframeBinMsgpack() error: NotBinarySlice: could not find 0xC4, 0xC5, 0xC6 in start of binary msgpack"
	default:
		return "UnknownUnframeError"
	}
}

// ninside returns the number of bytes inside/that follow the 2-5 byte
// binary msgpack header. The header frames the internal msgpack serialized
// object. ntotal returns the total number of bytes including the
// header bytes. The header is always a bin8/bin16/bin32 msgpack object
// itself, and so is 2-5 bytes extra, not counting the internal byte
// slice that makes up the internal msgp object. So there are two
// msgpack decoding steps to get a golang object back.
//
// UnframeBinMsgpack() works on just the minimal 2-5 bytes peek ahead
// needed to see how much to read next.
func UnframeBinMsgpack(p []byte) (ntotal int, ninside int, nheader int, err error) {

	if len(p) == 0 {
		err = NotEnoughBytes
		return
	}
	switch p[0] {
	case bin8:
		if len(p) < 2 {
			err = NotEnoughBytes
			return
		}
		ninside = int(p[1])
		nheader = 2
		ntotal = ninside + nheader
	case bin16:
		if len(p) < 3 {
			err = NotEnoughBytes
			return
		}
		ninside = int(binary.BigEndian.Uint16(p[1:3]))
		nheader = 3
		ntotal = ninside + nheader
	case bin32:
		if len(p) < 5 {
			err = NotEnoughBytes
			return
		}
		ninside = int(binary.BigEndian.Uint32(p[1:5]))
		nheader = 5
		ntotal = ninside + nheader
	default:
		fmt.Printf("p bytes = '%#v'\n", p[:5])
		fmt.Printf("p bytes = '%#v'/as string='%v'\n", p, string(p))
		err = NotBinarySlice
		panic(err)
	}
	return
}

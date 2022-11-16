package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	//snappy "github.com/glycerine/go-unsnap-stream"
	"github.com/glycerine/greenpack/msgp"
)

//go:generate greenpack

// HasherTyp tells us the type of a HasherElem.
type HasherTyp int

const (
	Command HasherTyp = 1
	Console HasherTyp = 2
	Image   HasherTyp = 4
)

// HasherElem are the "cells" of a notebook; in this
// case the elements of the HasherBook.Elems
type HasherElem struct {

	// Typ tells us which of the 3 types of content do we have.
	//
	// Althought the Typ bits are setup to allow
	// any mixture, we start with exact one type
	// in each Elem to unambiguously preserve the sequence order
	// in which they occur, so that replay is idempotent.
	//
	Typ HasherTyp `msg:"type" json:"type" zid:"0"`

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

	// where it was on disk;
	ImageHost string `msg:"imageHost" json:"imageHost" zid:"6"`
	ImagePath string `msg:"imagePath" json:"imagePath" zid:"7"`

	// ImageBy has png formatted graphic, referred to by ImageJSON and ImagePath;
	// checksummed by ImagePathHash.
	ImageBy []byte `msg:"imageBy" json:"imageBy" zid:"8"`

	// ImagePathHash = hash(ImageHost + ImagePath + ImageBy)
	ImagePathHash string `msg:"imagePathHash" json:"imagePathHash" zid:"9"`
}

type HasherBook struct {
	Elems []HasherElem `msg:"elems" json:"elems" zid:"0"`
}

type ByteSlice []byte

// read a HasherElem into e from r.
func LoadElem(r *msgp.Reader) (e *HasherElem, err error) {

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

	var ue HasherElem
	_, err = ue.UnmarshalMsg(bs2)
	if err != nil {
		return nil, fmt.Errorf("LoadElem() error on tk.UnmarshalMsg(): '%s'", err)
	}

	return &ue, nil
}

// Save tk as a framed msgpack message (where first few bytes are a []byte encoded
// to tell us the size of the rest of the bytes that follow. Those following
// bytes consist themselves of a msgpack serialized Tk.
func (e *HasherElem) SaveToSlice() ([]byte, error) {

	b, err := e.MarshalMsg(nil)
	if err != nil {
		return nil, fmt.Errorf("HasherElem.SaveToSlice() error on MarshalMsg: '%s'", err)
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

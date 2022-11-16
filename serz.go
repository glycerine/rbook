package main

import (
	"time"
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

	// png formatted graphic, referred to by ImageJSON and ImagePath;
	// checksummed by ImagePathHash.
	ImageBy []byte `msg:"imageBy" json:"imageBy" zid:"6"`

	// where it was on disk
	ImagePath     string `msg:"imagePath" json:"imagePath" zid:"7"`
	ImagePathHash string `msg:"imagePathHash" json:"imagePathHash" zid:"8"`
}

type HasherBook struct {
	Elems []HasherElem `msg:"elems" json:"elems" zid:"0"`
}

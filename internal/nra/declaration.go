// Package nra provides models and functionality for Bulgarian NRA tax declarations.
package nra

import (
	"encoding/xml"
)

// Declaration represents the main tax declaration (dec50).
type Declaration struct {
	XMLName   xml.Name   `xml:"dec50"`
	Part1     *struct{}  `xml:"part1,omitempty"`
	Part3     *Part3     `xml:"part3,omitempty"`
	Part4     *struct{}  `xml:"part4,omitempty"`
	Part5     *struct{}  `xml:"part5,omitempty"`
	Appendix5 *Appendix5 `xml:"app5,omitempty"`
	Appendix8 *Appendix8 `xml:"app8,omitempty"`
}

// NewDeclaration creates a new Declaration with the given appendices.
func NewDeclaration(app5 *Appendix5, app8 *Appendix8) *Declaration {
	issetApp5 := 0
	if app5 != nil {
		issetApp5 = 1
	}

	issetApp8 := 0
	if app8 != nil {
		issetApp8 = 1
	}

	return &Declaration{
		Part3:     NewPart3(issetApp5, issetApp8),
		Appendix5: app5,
		Appendix8: app8,
	}
}

// Part3 represents part 3 of the declaration (appendix flags).
type Part3 struct {
	XMLName      xml.Name `xml:"part3"`
	IssetApp1    int      `xml:"issetapp1"`
	IssetApp2    int      `xml:"issetapp2"`
	IssetApp3    int      `xml:"issetapp3"`
	IssetApp4    int      `xml:"issetapp4"`
	IssetApp5    int      `xml:"issetapp5"`
	IssetApp6    int      `xml:"issetapp6"`
	IssetApp7    int      `xml:"issetapp7"`
	IssetApp8    int      `xml:"issetapp8"`
	IssetApp9    int      `xml:"issetapp9"`
	IssetApp10   int      `xml:"issetapp10"`
	IssetApp11   int      `xml:"issetapp11"`
	IssetApp12   int      `xml:"issetapp12"`
	IssetApp13   int      `xml:"issetapp13"`
	HasNoIdoc    int      `xml:"hasnoidoc"`
	NoIdocs      *string  `xml:"noidocs,omitempty"`
	IssetApp2005 int      `xml:"issetapp2005"`
	IssetApp2006 int      `xml:"issetapp2006"`
	FinNum       *string  `xml:"finnum,omitempty"`
	FinData      *string  `xml:"findata,omitempty"`
	OtherDocs    *string  `xml:"otherdocs,omitempty"`
	PartStatus   int      `xml:"partstatus"`
}

// NewPart3 creates a new Part3 with the specified appendix flags.
func NewPart3(issetApp5, issetApp8 int) *Part3 {
	return &Part3{
		IssetApp5:  issetApp5,
		IssetApp8:  issetApp8,
		PartStatus: 1,
	}
}


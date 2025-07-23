// Package costing prices make ready work.
//
// The prices come out of a unit cost book that is effective dated, because an
// estimate written in March against the March book has to still reprice the
// same way in September when somebody queries the invoice. A book that is
// simply the current prices cannot answer that question at all.
package costing

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrNoPrice is returned when the book has no price for something.
var ErrNoPrice = errors.New("costing: no price in the book")

// Code identifies a priced item. Codes are stable across book versions, so a
// price change is a new entry on the same code rather than a new code.
type Code string

// The codes this utility's book carries.
const (
	CodeCrewHourLine    Code = "LBR-LINE-HR"
	CodeCrewHourComm    Code = "LBR-COMM-HR"
	CodeCrewHourDigger  Code = "LBR-DIG-HR"
	CodeMobilisation    Code = "LBR-MOB"
	CodeTrafficControl  Code = "LBR-TRAFFIC"
	CodePole40Class4    Code = "MAT-POLE-40-4"
	CodePole45Class3    Code = "MAT-POLE-45-3"
	CodePole50Class2    Code = "MAT-POLE-50-2"
	CodeAnchorRod       Code = "MAT-ANCHOR"
	CodeGuyStrand       Code = "MAT-GUY-FT"
	CodeGuyGuard        Code = "MAT-GUYGUARD"
	CodeThroughBolt     Code = "MAT-BOLT"
	CodeStandoffBracket Code = "MAT-STANDOFF"
	CodeDisposal        Code = "SVC-DISPOSAL"
	CodeEngineering     Code = "SVC-ENG-HR"
	CodeInspection      Code = "SVC-INSPECT"
)

// Entry is one price with the unit it is quoted in.
type Entry struct {
	Code        Code
	Description string
	Unit        string
	Price       unit.Money
}

// Book is an effective dated price list.
type Book struct {
	Name    string
	entries map[Code]*calendar.Timeline[Entry]
}

// NewBook builds an empty book.
func NewBook(name string) *Book {
	return &Book{Name: name, entries: map[Code]*calendar.Timeline[Entry]{}}
}

// Add files a price against a code from a date.
func (b *Book) Add(from calendar.Date, e Entry) error {
	if strings.TrimSpace(string(e.Code)) == "" {
		return fmt.Errorf("%w: an entry with no code", ErrNoPrice)
	}
	if e.Price < 0 {
		return fmt.Errorf("%w: %s is priced at %s", ErrNoPrice, e.Code, e.Price)
	}
	if strings.TrimSpace(e.Unit) == "" {
		return fmt.Errorf("%w: %s has no unit", ErrNoPrice, e.Code)
	}
	tl, ok := b.entries[e.Code]
	if !ok {
		tl = calendar.NewTimeline([]calendar.Effective[Entry]{})
		b.entries[e.Code] = tl
	}
	tl.Add(from, e)
	return nil
}

// Price returns what a code cost on a day.
func (b *Book) Price(c Code, on calendar.Date) (unit.Money, error) {
	e, err := b.Entry(c, on)
	if err != nil {
		return 0, err
	}
	return e.Price, nil
}

// Entry returns the whole book entry in force on a day.
func (b *Book) Entry(c Code, on calendar.Date) (Entry, error) {
	tl, ok := b.entries[c]
	if !ok {
		return Entry{}, fmt.Errorf("%w: %s is not in %s", ErrNoPrice, c, b.Name)
	}
	e, ok := tl.AsOf(on)
	if !ok {
		return Entry{}, fmt.Errorf("%w: %s had no price on %s", ErrNoPrice, c, on)
	}
	return e, nil
}

// Codes returns every code in the book, sorted.
func (b *Book) Codes() []Code {
	out := make([]Code, 0, len(b.entries))
	for c := range b.entries {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Versions returns how many prices a code has carried.
func (b *Book) Versions(c Code) int {
	tl, ok := b.entries[c]
	if !ok {
		return 0
	}
	return tl.Len()
}

// Standard returns the book this utility actually uses, with the 2025 prices
// and the increase that took effect on the first of January 2026.
func Standard() *Book {
	b := NewBook("summit valley joint use unit cost book")
	start := calendar.MustNew(2025, 1, 1)
	rise := calendar.MustNew(2026, 1, 1)

	type seed struct {
		code        Code
		description string
		unitName    string
		cents2025   int64
		cents2026   int64
	}
	for _, s := range []seed{
		{CodeCrewHourLine, "line crew, two workers and a bucket", "hour", 34500, 36200},
		{CodeCrewHourComm, "communication crew, two workers", "hour", 21000, 22100},
		{CodeCrewHourDigger, "digger derrick and operator", "hour", 28500, 30000},
		{CodeMobilisation, "mobilisation to a work location", "each", 47500, 49900},
		{CodeTrafficControl, "traffic control, flaggers and signs", "day", 89000, 93500},
		{CodePole40Class4, "40 foot class 4 southern pine, delivered", "each", 62500, 68000},
		{CodePole45Class3, "45 foot class 3 southern pine, delivered", "each", 81000, 88000},
		{CodePole50Class2, "50 foot class 2 southern pine, delivered", "each", 112000, 121000},
		{CodeAnchorRod, "screw anchor and rod, installed", "each", 41000, 43500},
		{CodeGuyStrand, "guy strand, 3/8 EHS", "foot", 420, 460},
		{CodeGuyGuard, "yellow guy guard", "each", 3800, 4100},
		{CodeThroughBolt, "through bolt with washers", "each", 1450, 1600},
		{CodeStandoffBracket, "standoff bracket", "each", 5900, 6300},
		{CodeDisposal, "pole removal and disposal", "each", 33000, 35500},
		{CodeEngineering, "make ready engineering", "hour", 15500, 16500},
		{CodeInspection, "post construction inspection", "each", 22000, 23500},
	} {
		_ = b.Add(start, Entry{Code: s.code, Description: s.description, Unit: s.unitName, Price: unit.Cents(s.cents2025)})
		_ = b.Add(rise, Entry{Code: s.code, Description: s.description, Unit: s.unitName, Price: unit.Cents(s.cents2026)})
	}
	return b
}

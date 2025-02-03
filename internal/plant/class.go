package plant

// Class is the ANSI strength class of a wood pole. The low numbers and the H
// classes are the strong ones, and the number is a circumference at the top,
// not a strength directly.
type Class string

// The classes this utility stocks.
const (
	ClassH6 Class = "H6"
	ClassH5 Class = "H5"
	ClassH4 Class = "H4"
	ClassH3 Class = "H3"
	ClassH2 Class = "H2"
	ClassH1 Class = "H1"
	Class1  Class = "1"
	Class2  Class = "2"
	Class3  Class = "3"
	Class4  Class = "4"
	Class5  Class = "5"
	Class6  Class = "6"
	Class7  Class = "7"
	Class9  Class = "9"
	Class10 Class = "10"
)

// classTop is the minimum circumference at the top of the pole, in inches, for
// each class. It is the same for every species.
var classTop = map[Class]int{
	ClassH6: 39, ClassH5: 37, ClassH4: 35, ClassH3: 33, ClassH2: 31, ClassH1: 29,
	Class1: 27, Class2: 25, Class3: 23, Class4: 21, Class5: 19,
	Class6: 17, Class7: 15, Class9: 15, Class10: 12,
}

// classRank orders the classes from strongest to weakest so that a replacement
// decision can ask for the next class up.
var classRank = []Class{
	ClassH6, ClassH5, ClassH4, ClassH3, ClassH2, ClassH1,
	Class1, Class2, Class3, Class4, Class5, Class6, Class7, Class9, Class10,
}

// KnownClass reports whether the class is one the tables cover.
func KnownClass(c Class) bool {
	_, ok := classTop[c]
	return ok
}

// TopCircumference returns the minimum circumference at the pole top in whole
// inches, and reports whether the class is known.
func TopCircumference(c Class) (int, bool) {
	v, ok := classTop[c]
	return v, ok
}

// Classes returns the classes in order from strongest to weakest.
func Classes() []Class {
	out := make([]Class, len(classRank))
	copy(out, classRank)
	return out
}

// Rank returns the position of a class in the strength order, and reports
// whether the class is known. A lower rank is a stronger pole.
func Rank(c Class) (int, bool) {
	for i, k := range classRank {
		if k == c {
			return i, true
		}
	}
	return 0, false
}

// Stronger returns the next class up from c and reports whether there is one.
// Class H6 is the top of the stock list and has nothing above it.
func Stronger(c Class) (Class, bool) {
	r, ok := Rank(c)
	if !ok || r == 0 {
		return c, false
	}
	return classRank[r-1], true
}

// StrongerThan reports whether a is a stronger class than b. An unknown class
// is never stronger than anything.
func StrongerThan(a, b Class) bool {
	ra, oka := Rank(a)
	rb, okb := Rank(b)
	if !oka || !okb {
		return false
	}
	return ra < rb
}

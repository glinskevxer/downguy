package clearance

import "strings"

// normaliseOwner folds an attacher name for comparison. Attacher names arrive
// from three different systems with three different ideas about case and
// trailing space, and comparing them raw is how one company ends up looking
// like two on the same pole.
func normaliseOwner(name string) string {
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

// SameAttacher reports whether two names refer to the same attacher.
func SameAttacher(a, b string) bool {
	na, nb := normaliseOwner(a), normaliseOwner(b)
	return na != "" && na == nb
}

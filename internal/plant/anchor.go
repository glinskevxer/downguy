package plant

// Anchor is a guy anchor in the ground with the guys that land on it. The
// loading package needs the lead and the direction to work out how much of the
// unbalanced pull the guy takes off the pole.
type Anchor struct {
	ID      string
	Pole    PoleID
	Lead    int
	Bearing int
	Guys    []Guy
}

// Guy is one strand from the pole to an anchor.
type Guy struct {
	ID       string
	Height   int
	Strength int
	Size     string
}

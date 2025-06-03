package plant

import (
	"fmt"
	"strings"
)

// Face is the quadrant of the pole a facility is mounted on. A pole is climbed
// on one face and framed on the others, and the code keeps a clear square up
// the climbing face so that a worker can get past everything on the pole
// without unhooking.
type Face string

// The four faces, plus the empty value for a facility whose face nobody has
// recorded.
const (
	FaceClimbing Face = "climbing"
	FaceStreet   Face = "street"
	FaceField    Face = "field"
	FaceRoad     Face = "road"
	FaceUnknown  Face = ""
)

var faceNames = map[Face]string{
	FaceClimbing: "climbing face",
	FaceStreet:   "street face",
	FaceField:    "field face",
	FaceRoad:     "road face",
}

// KnownFace reports whether the face is one of the four, which the empty value
// deliberately is not.
func KnownFace(f Face) bool {
	_, ok := faceNames[f]
	return ok
}

// Name returns the long form for a report.
func (f Face) Name() string {
	if n, ok := faceNames[f]; ok {
		return n
	}
	if f == FaceUnknown {
		return "face not recorded"
	}
	return "unknown face " + string(f)
}

// ParseFace reads a face name.
func ParseFace(text string) (Face, error) {
	t := Face(strings.ToLower(strings.TrimSpace(text)))
	if t == FaceUnknown {
		return FaceUnknown, nil
	}
	if KnownFace(t) {
		return t, nil
	}
	return "", fmt.Errorf("%w: unknown face %q", ErrBadAttachment, text)
}

// Obstructs reports whether a facility on this face gets in a climber's way.
// A facility whose face was never recorded is treated as not obstructing,
// because guessing that it does would fail half the poles in the system on no
// evidence.
func (f Face) Obstructs() bool { return f == FaceClimbing }

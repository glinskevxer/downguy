package plant

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// ErrBadAttachment covers every rejected attachment record.
var ErrBadAttachment = errors.New("plant: bad attachment record")

// Kind is what is actually bolted to the pole. The clearance rules do not care
// who owns a facility nearly as much as they care what it is, so this is the
// field most of the engine switches on.
type Kind string

// The facility kinds this system carries.
const (
	KindPrimary         Kind = "primary"
	KindSecondary       Kind = "secondary"
	KindNeutral         Kind = "neutral"
	KindStreetlight     Kind = "streetlight"
	KindServiceDrop     Kind = "service_drop"
	KindTransformer     Kind = "transformer"
	KindCommCable       Kind = "comm_cable"
	KindFiber           Kind = "fiber"
	KindCommDrop        Kind = "comm_drop"
	KindRiser           Kind = "riser"
	KindPowerSupply     Kind = "power_supply"
	KindEquipmentBox    Kind = "equipment_box"
	KindStandoffBracket Kind = "standoff"
)

var kindClass = map[Kind]string{
	KindPrimary:         "supply",
	KindSecondary:       "supply",
	KindNeutral:         "supply",
	KindStreetlight:     "supply",
	KindServiceDrop:     "supply",
	KindTransformer:     "supply",
	KindCommCable:       "communication",
	KindFiber:           "communication",
	KindCommDrop:        "communication",
	KindRiser:           "communication",
	KindPowerSupply:     "communication",
	KindEquipmentBox:    "communication",
	KindStandoffBracket: "communication",
}

// KnownKind reports whether the kind is one the rules cover.
func KnownKind(k Kind) bool {
	_, ok := kindClass[k]
	return ok
}

// IsSupply reports whether the facility is energised electric supply.
func (k Kind) IsSupply() bool { return kindClass[k] == "supply" }

// IsCommunication reports whether the facility sits in communication space.
func (k Kind) IsCommunication() bool { return kindClass[k] == "communication" }

// IsSpanCarrying reports whether the facility runs into the span rather than
// terminating on the pole. Only span carrying facilities have a sag and a mid
// span height.
func (k Kind) IsSpanCarrying() bool {
	switch k {
	case KindPrimary, KindSecondary, KindNeutral, KindServiceDrop,
		KindCommCable, KindFiber, KindCommDrop:
		return true
	}
	return false
}

// ParseKind reads a kind written with either an underscore or a space.
func ParseKind(text string) (Kind, error) {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.ReplaceAll(t, " ", "_")
	t = strings.ReplaceAll(t, "-", "_")
	if KnownKind(Kind(t)) {
		return Kind(t), nil
	}
	return "", fmt.Errorf("%w: unknown kind %q", ErrBadAttachment, text)
}

// String returns the kind code.
func (k Kind) String() string { return string(k) }

// Attachment is one facility on one pole at one height.
type Attachment struct {
	ID         string
	Pole       PoleID
	Owner      string
	Kind       Kind
	Height     unit.Height
	Authorized bool
	InForce    calendar.Range
	Voltage    int
	Face       Face
	Note       string
}

// Validate checks an attachment record.
func (a Attachment) Validate() error {
	if strings.TrimSpace(a.ID) == "" {
		return fmt.Errorf("%w: no id", ErrBadAttachment)
	}
	if _, err := ParsePoleID(string(a.Pole)); err != nil {
		return fmt.Errorf("%w: %s names pole %q", ErrBadAttachment, a.ID, a.Pole)
	}
	if strings.TrimSpace(a.Owner) == "" {
		return fmt.Errorf("%w: %s has no owner", ErrBadAttachment, a.ID)
	}
	if !KnownKind(a.Kind) {
		return fmt.Errorf("%w: %s has kind %q", ErrBadAttachment, a.ID, a.Kind)
	}
	if a.Height <= 0 {
		return fmt.Errorf("%w: %s is at %s", ErrBadAttachment, a.ID, a.Height)
	}
	if !a.InForce.Valid() {
		return fmt.Errorf("%w: %s is in force %s", ErrBadAttachment, a.ID, a.InForce)
	}
	if a.Kind.IsCommunication() && a.Voltage != 0 {
		return fmt.Errorf("%w: %s is communication but carries %d volts", ErrBadAttachment, a.ID, a.Voltage)
	}
	if a.Kind == KindPrimary && a.Voltage <= 750 {
		return fmt.Errorf("%w: %s is primary at %d volts", ErrBadAttachment, a.ID, a.Voltage)
	}
	if a.Face != FaceUnknown && !KnownFace(a.Face) {
		return fmt.Errorf("%w: %s is mounted on the %q", ErrBadAttachment, a.ID, a.Face)
	}
	return nil
}

// ActiveOn reports whether the attachment was on the pole on the day.
func (a Attachment) ActiveOn(d calendar.Date) bool { return a.InForce.Contains(d) }

// String prints the attachment the way a survey sheet lists it.
func (a Attachment) String() string {
	auth := "unauthorized"
	if a.Authorized {
		auth = "authorized"
	}
	return fmt.Sprintf("%s %s %s at %s (%s)", a.ID, a.Owner, a.Kind, a.Height, auth)
}

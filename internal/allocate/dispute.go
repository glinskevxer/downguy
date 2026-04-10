package allocate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/unit"
)

// An attacher that disagrees with a make ready invoice has a window to say so
// and a set of grounds it can say it on. What happens next is a credit against
// the invoice or a refusal, and both have to be written down, because the next
// stop is the regulator and the regulator reads the file.

// Ground is why a charge is being disputed.
type Ground string

// The grounds the agreement allows.
const (
	// GroundAttribution says the work was not made necessary by this
	// attachment.
	GroundAttribution Ground = "attribution"
	// GroundPricing says the work is priced above the book.
	GroundPricing Ground = "pricing"
	// GroundScope says the work goes beyond what the code requires.
	GroundScope Ground = "scope"
	// GroundDuplication says the same work is billed twice.
	GroundDuplication Ground = "duplication"
)

var groundNames = map[Ground]string{
	GroundAttribution: "the work was not made necessary by this attachment",
	GroundPricing:     "the work is priced above the unit cost book",
	GroundScope:       "the work goes beyond what the code requires",
	GroundDuplication: "the same work is billed twice",
}

// KnownGround reports whether the ground is one the agreement allows.
func KnownGround(g Ground) bool {
	_, ok := groundNames[g]
	return ok
}

// Name returns the long form the agreement uses.
func (g Ground) Name() string {
	if n, ok := groundNames[g]; ok {
		return n
	}
	return "unrecognised ground " + string(g)
}

// Grounds returns the allowed grounds, sorted.
func Grounds() []Ground {
	out := make([]Ground, 0, len(groundNames))
	for g := range groundNames {
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// DisputeWindowDays is how long after an invoice an attacher has to dispute
// it. Past that the charge stands however good the argument is, which is what
// makes the window worth writing down.
const DisputeWindowDays = 30

// Outcome is what happened to a dispute.
type Outcome string

// The outcomes.
const (
	OutcomeOpen      Outcome = "open"
	OutcomeUpheld    Outcome = "upheld"
	OutcomeRefused   Outcome = "refused"
	OutcomeWithdrawn Outcome = "withdrawn"
)

// KnownOutcome reports whether the outcome is one the ledger handles.
func KnownOutcome(o Outcome) bool {
	switch o {
	case OutcomeOpen, OutcomeUpheld, OutcomeRefused, OutcomeWithdrawn:
		return true
	}
	return false
}

// Dispute is one attacher's objection to one charge.
type Dispute struct {
	ID       string
	Invoice  string
	ItemID   string
	Party    string
	Ground   Ground
	Raised   calendar.Date
	Claimed  unit.Money
	Outcome  Outcome
	Decided  calendar.Date
	Credited unit.Money
	Reason   string
}

// Validate checks a dispute record.
func (d Dispute) Validate() error {
	switch {
	case strings.TrimSpace(d.ID) == "":
		return fmt.Errorf("%w: a dispute with no id", ErrBadParty)
	case strings.TrimSpace(d.Invoice) == "":
		return fmt.Errorf("%w: %s disputes no invoice", ErrBadParty, d.ID)
	case strings.TrimSpace(d.Party) == "":
		return fmt.Errorf("%w: %s names no party", ErrBadParty, d.ID)
	case !KnownGround(d.Ground):
		return fmt.Errorf("%w: %s is raised on %q", ErrBadParty, d.ID, d.Ground)
	case d.Raised.IsZero():
		return fmt.Errorf("%w: %s has no date", ErrBadParty, d.ID)
	case d.Claimed <= 0:
		return fmt.Errorf("%w: %s claims %s", ErrBadParty, d.ID, d.Claimed)
	case !KnownOutcome(d.Outcome):
		return fmt.Errorf("%w: %s ended %q", ErrBadParty, d.ID, d.Outcome)
	}
	if d.Outcome == OutcomeOpen {
		if !d.Decided.IsZero() || !d.Credited.IsZero() {
			return fmt.Errorf("%w: %s is open but carries a decision", ErrBadParty, d.ID)
		}
		return nil
	}
	if d.Decided.IsZero() {
		return fmt.Errorf("%w: %s is %s with no decision date", ErrBadParty, d.ID, d.Outcome)
	}
	if d.Decided.Before(d.Raised) {
		return fmt.Errorf("%w: %s was decided before it was raised", ErrBadParty, d.ID)
	}
	if strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("%w: %s is %s with no reason written down", ErrBadParty, d.ID, d.Outcome)
	}
	if d.Credited < 0 || d.Credited > d.Claimed {
		return fmt.Errorf("%w: %s credits %s against a claim of %s", ErrBadParty, d.ID, d.Credited, d.Claimed)
	}
	if d.Outcome != OutcomeUpheld && !d.Credited.IsZero() {
		return fmt.Errorf("%w: %s is %s and still credits %s", ErrBadParty, d.ID, d.Outcome, d.Credited)
	}
	if d.Outcome == OutcomeUpheld && d.Credited.IsZero() {
		return fmt.Errorf("%w: %s is upheld and credits nothing", ErrBadParty, d.ID)
	}
	return nil
}

// InTime reports whether the dispute was raised inside the window from the
// invoice date.
func (d Dispute) InTime(invoiced calendar.Date) bool {
	if invoiced.IsZero() || d.Raised.IsZero() {
		return false
	}
	if d.Raised.Before(invoiced) {
		return false
	}
	return invoiced.DaysUntil(d.Raised) <= DisputeWindowDays
}

// Open reports whether the dispute is still to be decided.
func (d Dispute) Open() bool { return d.Outcome == OutcomeOpen }

// String prints the dispute the way a ledger lists it.
func (d Dispute) String() string {
	return fmt.Sprintf("%s %s on %s: %s claiming %s, %s",
		d.ID, d.Party, d.Invoice, d.Ground, d.Claimed, d.Outcome)
}

// Settlement is one invoice after its disputes are applied.
type Settlement struct {
	Invoice  Invoice
	Disputes []Dispute
	Credited unit.Money
	Withheld unit.Money
	Payable  unit.Money
}

// Settle applies a set of disputes to an invoice. An upheld dispute credits
// the invoice outright; an open one withholds its claim until it is decided,
// which is not the same thing and is the distinction the ledger has to keep.
func Settle(invoice Invoice, disputes []Dispute) (*Settlement, error) {
	if err := invoice.Validate(); err != nil {
		return nil, err
	}
	out := &Settlement{Invoice: invoice}
	for _, d := range disputes {
		if d.Invoice != invoice.Number {
			continue
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		if !d.InTime(invoice.Issued) {
			continue
		}
		out.Disputes = append(out.Disputes, d)
		switch d.Outcome {
		case OutcomeUpheld:
			out.Credited = out.Credited.Add(d.Credited)
		case OutcomeOpen:
			out.Withheld = out.Withheld.Add(d.Claimed)
		}
	}
	sort.SliceStable(out.Disputes, func(i, j int) bool { return out.Disputes[i].ID < out.Disputes[j].ID })
	out.Payable = invoice.Total.Sub(out.Credited).Sub(out.Withheld)
	if out.Payable < 0 {
		out.Payable = 0
	}
	return out, nil
}

// OpenDisputes returns the ones still to be decided.
func (s *Settlement) OpenDisputes() []Dispute {
	var out []Dispute
	for _, d := range s.Disputes {
		if d.Open() {
			out = append(out, d)
		}
	}
	return out
}

// Settled reports whether nothing is still being argued about.
func (s *Settlement) Settled() bool { return len(s.OpenDisputes()) == 0 }

// Summary writes the line a statement carries.
func (s *Settlement) Summary() string {
	return fmt.Sprintf("%s: %s billed, %s credited, %s withheld, %s payable",
		s.Invoice.Number, s.Invoice.Total, s.Credited, s.Withheld, s.Payable)
}

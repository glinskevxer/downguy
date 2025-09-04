package rental

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Sanction is what an unauthorised attachment costs beyond the rent. There are
// only three shapes and the difference between them is how long the attachment
// has been up, not how bad it looks.
type Sanction string

// The sanctions the joint use agreement provides for.
const (
	// SanctionNone applies where the attacher self reports before an audit
	// finds it.
	SanctionNone Sanction = "none"
	// SanctionBackRent charges the rent that should have been paid.
	SanctionBackRent Sanction = "back_rent"
	// SanctionPenalty adds a multiple of the rent on top of the back rent.
	SanctionPenalty Sanction = "penalty"
)

// The agreement's figures.
const (
	// BackRentYears is how far back the agreement allows the owner to bill,
	// regardless of how long the attachment has actually been there.
	BackRentYears = 5
	// PenaltyMultiple is the multiple of one year's rent added as a penalty.
	PenaltyMultiple = 5
	// SelfReportGraceDays is how long after an audit notice an attacher can
	// still self report and avoid the penalty.
	SelfReportGraceDays = 60
)

// Discovery is how an unauthorised attachment came to light.
type Discovery string

// The two ways.
const (
	DiscoverySelfReported Discovery = "self_reported"
	DiscoveryAudit        Discovery = "audit"
)

// KnownDiscovery reports whether the discovery is one the agreement covers.
func KnownDiscovery(d Discovery) bool {
	return d == DiscoverySelfReported || d == DiscoveryAudit
}

// Claim is the owner's claim against one unauthorised attachment.
type Claim struct {
	AttachmentID string
	Pole         plant.PoleID
	Owner        string
	Discovery    Discovery
	Found        calendar.Date
	FirstUp      calendar.Date
	Sanction     Sanction
	BackYears    []calendar.RateYear
	BackRent     unit.Money
	Penalty      unit.Money
	Total        unit.Money
	Note         string
}

// SanctionFor decides which sanction applies. A self reported attachment
// inside the grace period carries none; anything an audit turns up carries the
// penalty as well as the back rent.
func SanctionFor(d Discovery, found, noticed calendar.Date) (Sanction, error) {
	if !KnownDiscovery(d) {
		return "", fmt.Errorf("%w: %q is not a way of finding one", ErrBadRate, d)
	}
	if d == DiscoveryAudit {
		return SanctionPenalty, nil
	}
	if noticed.IsZero() {
		return SanctionNone, nil
	}
	if found.After(noticed.AddDays(SelfReportGraceDays)) {
		return SanctionBackRent, nil
	}
	return SanctionNone, nil
}

// AssessClaim works out what an unauthorised attachment owes. The back rent
// runs from the later of when it went up and the earliest year the agreement
// reaches, and it is charged at each year's own rate rather than at today's.
func AssessClaim(o Occupancy, d Discovery, found, noticed calendar.Date, attachers int, assign Assignment) (*Claim, error) {
	if strings.TrimSpace(o.AttachmentID) == "" {
		return nil, fmt.Errorf("%w: a claim against no attachment", ErrBadRate)
	}
	if found.IsZero() {
		return nil, fmt.Errorf("%w: %s has no discovery date", ErrBadRate, o.AttachmentID)
	}
	sanction, err := SanctionFor(d, found, noticed)
	if err != nil {
		return nil, err
	}
	c := &Claim{
		AttachmentID: o.AttachmentID, Pole: o.Pole, Owner: o.Owner,
		Discovery: d, Found: found, FirstUp: o.Active.From, Sanction: sanction,
	}
	if sanction == SanctionNone {
		c.Note = "self reported inside the grace period, rent runs from discovery"
		return c, nil
	}

	formula := assign.FormulaFor(o.Owner)
	space := StandardSpace()
	earliest := found.Year() - BackRentYears + 1
	if !o.Active.From.IsZero() && o.Active.From.Year() > earliest {
		earliest = o.Active.From.Year()
	}
	for year := earliest; year <= found.Year(); year++ {
		rate, err := Compute(calendar.RateYear(year), formula, space, StandardOccupied, attachers)
		if err != nil {
			// A year with no filing cannot be billed. Skip it and say so,
			// rather than substituting a rate the utility never filed.
			c.Note = fmt.Sprintf("no filed rate for %d, so that year is not claimed", year)
			continue
		}
		c.BackYears = append(c.BackYears, calendar.RateYear(year))
		c.BackRent = c.BackRent.Add(rate.Prorated(o.Active))
	}
	c.Total = c.BackRent

	if sanction == SanctionPenalty && len(c.BackYears) > 0 {
		latest := c.BackYears[len(c.BackYears)-1]
		rate, err := Compute(latest, formula, space, StandardOccupied, attachers)
		if err != nil {
			return nil, err
		}
		c.Penalty = rate.Annual.Times(PenaltyMultiple)
		c.Total = c.Total.Add(c.Penalty)
	}
	return c, nil
}

// AssessAll works out claims for every unauthorised attachment in an
// inventory, sorted by pole and attachment.
func AssessAll(inv *Inventory, d Discovery, found, noticed calendar.Date, assign Assignment) ([]Claim, error) {
	if inv == nil {
		return nil, fmt.Errorf("%w: no inventory", ErrBadRate)
	}
	var out []Claim
	for _, o := range inv.Unauthorized() {
		c, err := AssessClaim(o, d, found, noticed, inv.AttachersOn(o.Pole), assign)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pole != out[j].Pole {
			return out[i].Pole < out[j].Pole
		}
		return out[i].AttachmentID < out[j].AttachmentID
	})
	return out, nil
}

// ClaimTotal adds up a set of claims.
func ClaimTotal(claims []Claim) unit.Money {
	var total unit.Money
	for _, c := range claims {
		total = total.Add(c.Total)
	}
	return total
}

// String prints the claim the way a demand letter opens.
func (c Claim) String() string {
	return fmt.Sprintf("%s on %s, %s: %s back rent plus %s penalty, %s in all",
		c.AttachmentID, c.Pole, c.Sanction, c.BackRent, c.Penalty, c.Total)
}

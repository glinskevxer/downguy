package allocate

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Payer says who a cost lands on. It is deliberately not the same type as
// Role: a party has a role on the application, and a cost has a payer, and
// they are decided by different things.
type Payer string

// The payers a cost can land on.
const (
	PayerApplicant Payer = "applicant"
	PayerOwner     Payer = "owner"
	PayerExisting  Payer = "existing_attacher"
	PayerShared    Payer = "shared"
)

// KnownPayer reports whether the payer is one the split handles.
func KnownPayer(p Payer) bool {
	switch p {
	case PayerApplicant, PayerOwner, PayerExisting, PayerShared:
		return true
	}
	return false
}

// Policy is the set of rules deciding who pays. It is a value rather than a
// constant because two states this utility operates in write the pre existing
// violation rule differently, and the difference is worth several thousand
// dollars an application.
type Policy struct {
	Name string
	// ApplicantPaysCaused bills the applicant for what its attachment makes
	// necessary. Every policy sets this; it is here so the rule is visible
	// rather than assumed.
	ApplicantPaysCaused bool
	// ApplicantPaysAggravated bills the applicant for a condition it made
	// worse but did not create.
	ApplicantPaysAggravated bool
	// PreExistingPayer says who picks up a violation that was already there.
	PreExistingPayer Payer
	// OwnerPaysStructural bills the pole owner for a pole replacement where
	// the pole was already overloaded before the application.
	OwnerPaysStructural bool
	// ApplicantShare is the fraction of a pre existing condition that falls
	// on the applicant when the payer is shared.
	ApplicantShare unit.BasisPoints
}

// Validate checks a policy is usable.
func (p Policy) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("%w: a policy needs a name", ErrBadParty)
	}
	if !KnownPayer(p.PreExistingPayer) {
		return fmt.Errorf("%w: %s bills pre existing work to %q", ErrBadParty, p.Name, p.PreExistingPayer)
	}
	if p.PreExistingPayer == PayerShared {
		if p.ApplicantShare <= 0 || p.ApplicantShare >= unit.FullRate {
			return fmt.Errorf("%w: %s shares pre existing work at %s", ErrBadParty, p.Name, p.ApplicantShare)
		}
	}
	return nil
}

// FederalPolicy is the rule as the federal regulator writes it: a new attacher
// pays for what it causes and for what it makes worse, and the owner or the
// existing attacher picks up everything that was already wrong.
func FederalPolicy() Policy {
	return Policy{
		Name:                    "federal",
		ApplicantPaysCaused:     true,
		ApplicantPaysAggravated: true,
		PreExistingPayer:        PayerExisting,
		OwnerPaysStructural:     true,
	}
}

// StatePolicy is the variant one of this utility's two states applies, where a
// pre existing condition on a pole an applicant is joining is shared, on the
// theory that the applicant gets the benefit of the corrected pole.
func StatePolicy() Policy {
	return Policy{
		Name:                    "state commission",
		ApplicantPaysCaused:     true,
		ApplicantPaysAggravated: true,
		PreExistingPayer:        PayerShared,
		ApplicantShare:          unit.Percent(25),
		OwnerPaysStructural:     true,
	}
}

// Policies returns the policies this system knows, sorted by name.
func Policies() []Policy {
	out := []Policy{FederalPolicy(), StatePolicy()}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PolicyNamed returns a policy by name.
func PolicyNamed(name string) (Policy, error) {
	for _, p := range Policies() {
		if p.Name == name {
			return p, nil
		}
	}
	return Policy{}, fmt.Errorf("%w: no policy named %q", ErrBadParty, name)
}

// Share is how one cost divides between payers, in basis points. The shares
// always add to exactly one hundred percent; a split that does not is a bug
// rather than a rounding difference, because the remainder is assigned rather
// than dropped.
type Share map[Payer]unit.BasisPoints

// Total returns the sum of the shares, which should always be the full rate.
func (s Share) Total() unit.BasisPoints {
	var total unit.BasisPoints
	for _, v := range s {
		total += v
	}
	return total
}

// Payers returns the payers in the share, sorted.
func (s Share) Payers() []Payer {
	out := make([]Payer, 0, len(s))
	for p := range s {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ShareFor returns how a cost arising from one finding divides under a policy.
func (p Policy) ShareFor(blame clearance.Blame) (Share, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	switch blame {
	case clearance.BlameApplicant:
		if p.ApplicantPaysCaused {
			return Share{PayerApplicant: unit.FullRate}, nil
		}
		return Share{PayerOwner: unit.FullRate}, nil
	case clearance.BlameAggravated:
		if p.ApplicantPaysAggravated {
			return Share{PayerApplicant: unit.FullRate}, nil
		}
		return Share{p.PreExistingPayer: unit.FullRate}, nil
	case clearance.BlamePreExisting:
		if p.PreExistingPayer == PayerShared {
			return Share{
				PayerApplicant: p.ApplicantShare,
				PayerExisting:  unit.FullRate - p.ApplicantShare,
			}, nil
		}
		return Share{p.PreExistingPayer: unit.FullRate}, nil
	case clearance.BlameUnattributed:
		return nil, fmt.Errorf("%w: an unattributed finding cannot be billed", ErrBadParty)
	}
	return nil, fmt.Errorf("%w: %q is not an attribution", ErrBadParty, blame)
}

// Divide splits an amount according to a share, assigning the rounding
// remainder to the payer with the largest share so that the parts always add
// back to the whole.
func Divide(amount unit.Money, s Share) (map[Payer]unit.Money, error) {
	if s.Total() != unit.FullRate {
		return nil, fmt.Errorf("%w: shares add to %s", ErrBadParty, s.Total())
	}
	out := map[Payer]unit.Money{}
	var assigned unit.Money
	payers := s.Payers()
	for _, p := range payers {
		part := s[p].Of(amount)
		out[p] = part
		assigned = assigned.Add(part)
	}
	if remainder := amount.Sub(assigned); !remainder.IsZero() {
		biggest := payers[0]
		for _, p := range payers[1:] {
			if s[p] > s[biggest] {
				biggest = p
			}
		}
		out[biggest] = out[biggest].Add(remainder)
	}
	return out, nil
}

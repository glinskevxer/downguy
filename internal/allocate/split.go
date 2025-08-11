package allocate

import (
	"fmt"
	"sort"

	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/unit"
)

// Charge is one priced work item landed on one party.
type Charge struct {
	ItemID  string
	Pole    string
	Payer   Payer
	Party   string
	Blame   clearance.Blame
	Amount  unit.Money
	Because string
}

// Split is the finished allocation for one application.
type Split struct {
	Application string
	Policy      Policy
	Charges     []Charge
	Unallocated []string
	Total       unit.Money
}

// Allocate lands every priced item on a party. It needs three things the
// engineering does not carry: the roster, so a name resolves to a payer; the
// plan, so an item resolves to the findings behind it; and the attributed
// findings, so a finding resolves to a blame.
func Allocate(app string, p Policy, roster *Roster, plan *makeready.Plan, e *costing.Estimate, findings clearance.Findings) (*Split, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if roster == nil {
		return nil, fmt.Errorf("%w: no roster", ErrBadParty)
	}
	if err := roster.Validate(); err != nil {
		return nil, err
	}
	if plan == nil || e == nil {
		return nil, fmt.Errorf("%w: allocation needs a plan and an estimate", ErrBadParty)
	}

	blameByKey := map[string]clearance.Blame{}
	for _, f := range findings {
		blameByKey[f.Key()] = f.Blame
	}
	ownerOf := map[string]makeready.Item{}
	for _, item := range plan.Items {
		ownerOf[item.ID] = item
	}

	s := &Split{Application: app, Policy: p}
	for _, item := range plan.Items {
		amount := e.TotalForItem(item.ID)
		if amount.IsZero() {
			continue
		}
		blame, ok := blameOf(item, blameByKey)
		if !ok {
			s.Unallocated = append(s.Unallocated, item.ID)
			continue
		}
		share, err := p.ShareFor(blame)
		if err != nil {
			s.Unallocated = append(s.Unallocated, item.ID)
			continue
		}
		parts, err := Divide(amount, share)
		if err != nil {
			return nil, err
		}
		for _, payer := range share.Payers() {
			part := parts[payer]
			if part.IsZero() {
				continue
			}
			s.Charges = append(s.Charges, Charge{
				ItemID: item.ID, Pole: item.Pole, Payer: payer,
				Party: partyFor(roster, payer, item.Owner), Blame: blame,
				Amount: part, Because: item.Detail,
			})
			s.Total = s.Total.Add(part)
		}
	}
	sort.SliceStable(s.Charges, func(i, j int) bool {
		if s.Charges[i].ItemID != s.Charges[j].ItemID {
			return s.Charges[i].ItemID < s.Charges[j].ItemID
		}
		return s.Charges[i].Payer < s.Charges[j].Payer
	})
	sort.Strings(s.Unallocated)
	return s, nil
}

// blameOf returns the attribution behind a work item. An item that cites more
// than one finding takes the heaviest attribution of them, because the work is
// done once and somebody has to own all of it.
func blameOf(item makeready.Item, byKey map[string]clearance.Blame) (clearance.Blame, bool) {
	best := clearance.BlameUnattributed
	found := false
	for _, key := range item.Because {
		b, ok := byKey[key]
		if !ok {
			continue
		}
		found = true
		if heavier(b, best) {
			best = b
		}
	}
	if !found || best == clearance.BlameUnattributed {
		return clearance.BlameUnattributed, false
	}
	return best, true
}

var blameWeight = map[clearance.Blame]int{
	clearance.BlameApplicant:    3,
	clearance.BlameAggravated:   2,
	clearance.BlamePreExisting:  1,
	clearance.BlameUnattributed: 0,
}

func heavier(a, b clearance.Blame) bool { return blameWeight[a] > blameWeight[b] }

// partyFor turns a payer into the company that actually gets the invoice.
func partyFor(r *Roster, payer Payer, itemOwner string) string {
	switch payer {
	case PayerApplicant:
		if p, ok := r.Applicant(); ok {
			return p.Name
		}
	case PayerOwner:
		if p, ok := r.Owner(); ok {
			return p.Name
		}
	case PayerExisting:
		if p, ok := r.Lookup(itemOwner); ok && p.Role == RoleExisting {
			return p.Name
		}
		if p, ok := r.Owner(); ok {
			// An existing violation nobody will claim falls back to the owner,
			// which is the same answer the utility reaches by hand.
			return p.Name
		}
	}
	return "UNASSIGNED"
}

// ByParty totals the split by company.
func (s *Split) ByParty() map[string]unit.Money {
	out := map[string]unit.Money{}
	for _, c := range s.Charges {
		out[c.Party] = out[c.Party].Add(c.Amount)
	}
	return out
}

// ByPayer totals the split by payer role.
func (s *Split) ByPayer() map[Payer]unit.Money {
	out := map[Payer]unit.Money{}
	for _, c := range s.Charges {
		out[c.Payer] = out[c.Payer].Add(c.Amount)
	}
	return out
}

// Parties returns every company with a charge, sorted.
func (s *Split) Parties() []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range s.Charges {
		if seen[c.Party] {
			continue
		}
		seen[c.Party] = true
		out = append(out, c.Party)
	}
	sort.Strings(out)
	return out
}

// ForParty returns the charges landing on one company.
func (s *Split) ForParty(name string) []Charge {
	var out []Charge
	for _, c := range s.Charges {
		if Fold(c.Party) == Fold(name) {
			out = append(out, c)
		}
	}
	return out
}

// String prints the one line summary a cover letter opens with.
func (s *Split) String() string {
	return fmt.Sprintf("%s under the %s policy: %s across %d parties, %d items unallocated",
		s.Application, s.Policy.Name, s.Total, len(s.Parties()), len(s.Unallocated))
}

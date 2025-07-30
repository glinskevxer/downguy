// Package allocate decides who pays for make ready.
//
// The engineering says what has to be done. This package says who is billed
// for it, and it is the part attachers actually argue about. The governing
// principle is simple and the application of it is not: a new attacher pays
// for the work its own attachment makes necessary, and it does not pay to fix
// conditions that were already there when it applied.
package allocate

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrBadParty covers every rejected party or role.
var ErrBadParty = errors.New("allocate: bad party")

// Role is what a party is to this application.
type Role string

// The roles on a joint use application.
const (
	// RoleOwner is the utility that owns the poles.
	RoleOwner Role = "owner"
	// RoleApplicant is the attacher asking to go up.
	RoleApplicant Role = "applicant"
	// RoleExisting is an attacher already on the pole.
	RoleExisting Role = "existing"
	// RoleUnknown is a facility nobody has claimed, which happens more often
	// than anybody would like.
	RoleUnknown Role = "unknown"
)

var roleNames = map[Role]string{
	RoleOwner:     "pole owner",
	RoleApplicant: "new attacher",
	RoleExisting:  "existing attacher",
	RoleUnknown:   "unidentified",
}

// KnownRole reports whether the role is one the policy handles.
func KnownRole(r Role) bool {
	_, ok := roleNames[r]
	return ok
}

// Name returns the long form for a report.
func (r Role) Name() string {
	if n, ok := roleNames[r]; ok {
		return n
	}
	return "unknown role " + string(r)
}

// ParseRole reads a role name.
func ParseRole(text string) (Role, error) {
	t := Role(strings.ToLower(strings.TrimSpace(text)))
	if KnownRole(t) {
		return t, nil
	}
	return "", fmt.Errorf("%w: unknown role %q", ErrBadParty, text)
}

// Party is one company on the application.
type Party struct {
	Name    string
	Role    Role
	Account string
	Contact string
}

// Validate checks a party record.
func (p Party) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: a party with no name", ErrBadParty)
	}
	if !KnownRole(p.Role) {
		return fmt.Errorf("%w: %s is a %q", ErrBadParty, p.Name, p.Role)
	}
	if p.Role != RoleUnknown && strings.TrimSpace(p.Account) == "" {
		return fmt.Errorf("%w: %s has no account number", ErrBadParty, p.Name)
	}
	return nil
}

// Roster is every party on one application, indexed by folded name so that the
// three systems this data arrives from can spell a company three ways and
// still resolve to one payer.
type Roster struct {
	parties map[string]Party
}

// NewRoster builds a roster.
func NewRoster() *Roster { return &Roster{parties: map[string]Party{}} }

// Add files a party, replacing any earlier record of the same name.
func (r *Roster) Add(p Party) error {
	if err := p.Validate(); err != nil {
		return err
	}
	key := Fold(p.Name)
	if existing, ok := r.parties[key]; ok && existing.Role != p.Role {
		return fmt.Errorf("%w: %s is already on this application as a %s", ErrBadParty, p.Name, existing.Role)
	}
	r.parties[key] = p
	return nil
}

// Lookup returns the party under a name, however it is spelled.
func (r *Roster) Lookup(name string) (Party, bool) {
	p, ok := r.parties[Fold(name)]
	return p, ok
}

// RoleOf returns the role a name carries, or RoleUnknown when the roster has
// never heard of it.
func (r *Roster) RoleOf(name string) Role {
	if p, ok := r.Lookup(name); ok {
		return p.Role
	}
	return RoleUnknown
}

// InRole returns every party in a role, sorted by name.
func (r *Roster) InRole(role Role) []Party {
	var out []Party
	for _, p := range r.parties {
		if p.Role == role {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Applicant returns the single applicant and reports whether there is exactly
// one. An application with two applicants or none is a data problem, not a
// case to allocate.
func (r *Roster) Applicant() (Party, bool) {
	found := r.InRole(RoleApplicant)
	if len(found) != 1 {
		return Party{}, false
	}
	return found[0], true
}

// Owner returns the pole owner and reports whether there is exactly one.
func (r *Roster) Owner() (Party, bool) {
	found := r.InRole(RoleOwner)
	if len(found) != 1 {
		return Party{}, false
	}
	return found[0], true
}

// Names returns every party name, sorted.
func (r *Roster) Names() []string {
	out := make([]string, 0, len(r.parties))
	for _, p := range r.parties {
		out = append(out, p.Name)
	}
	sort.Strings(out)
	return out
}

// Len returns how many parties are on the application.
func (r *Roster) Len() int { return len(r.parties) }

// Validate checks the roster hangs together well enough to allocate against.
func (r *Roster) Validate() error {
	if _, ok := r.Owner(); !ok {
		return fmt.Errorf("%w: an application needs exactly one pole owner", ErrBadParty)
	}
	if _, ok := r.Applicant(); !ok {
		return fmt.Errorf("%w: an application needs exactly one applicant", ErrBadParty)
	}
	return nil
}

// Fold normalises a company name for comparison.
func Fold(name string) string {
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

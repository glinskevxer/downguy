package makeready

import (
	"fmt"
	"sort"
	"strings"
)

// A plan is what has to happen. A packet is what one crew is handed on one
// day. They are not the same list: work on a pole has to be done in stage
// order by whoever is competent to do it, and a line crew and a communication
// crew cannot be on the same pole at the same time.

// Discipline is which crew does a piece of work.
type Discipline string

// The two crews this utility sends out, and the case where nobody is assigned.
const (
	// DisciplineLine is the electric line crew: anything that touches supply
	// space or the structure itself.
	DisciplineLine Discipline = "line"
	// DisciplineComm is the communication crew: anything in communication
	// space.
	DisciplineComm Discipline = "communication"
	// DisciplineUnassigned is work nobody has been given.
	DisciplineUnassigned Discipline = "unassigned"
)

// KnownDiscipline reports whether the crew is one that gets sent out.
func KnownDiscipline(d Discipline) bool {
	return d == DisciplineLine || d == DisciplineComm || d == DisciplineUnassigned
}

// DisciplineFor returns which crew does an action.
func DisciplineFor(a Action) Discipline {
	switch a {
	case ActionReplacePole, ActionAddGuy:
		return DisciplineLine
	case ActionRaise, ActionLower, ActionTransfer, ActionRemove,
		ActionResag, ActionRelocateEquipment:
		return DisciplineComm
	}
	return DisciplineUnassigned
}

// Packet is one crew's work on one pole, in the order they do it.
type Packet struct {
	Pole        string
	Discipline  Discipline
	Items       []Item
	Owners      []string
	NeedsPole   bool
	NeedsOutage bool
}

// Size returns how many items are in the packet.
func (p Packet) Size() int { return len(p.Items) }

// String prints the packet header a crew sheet carries.
func (p Packet) String() string {
	extra := ""
	if p.NeedsOutage {
		extra = ", outage required"
	}
	return fmt.Sprintf("%s %s crew: %d items%s", p.Pole, p.Discipline, len(p.Items), extra)
}

// Packets splits a plan into what each crew is handed. Work is grouped by pole
// and then by crew, and within a packet it stays in stage order, because that
// ordering is the whole reason the plan bothered to compute one.
//
// A pole that needs structure work gets its line packet marked as needing an
// outage, and every communication packet on that pole is marked as waiting on
// it: the cable cannot be transferred to a pole that is not in the ground yet.
func Packets(plan *Plan) []Packet {
	if plan == nil {
		return nil
	}
	type key struct {
		pole       string
		discipline Discipline
	}
	grouped := map[key][]Item{}
	structural := map[string]bool{}
	for _, item := range plan.Items {
		d := DisciplineFor(item.Action)
		grouped[key{item.Pole, d}] = append(grouped[key{item.Pole, d}], item)
		if item.Action.PoleWork() {
			structural[item.Pole] = true
		}
	}

	keys := make([]key, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].pole != keys[j].pole {
			return keys[i].pole < keys[j].pole
		}
		// The line crew goes first on any pole they share, because everything
		// the communication crew does hangs off what the line crew leaves.
		return disciplineRank(keys[i].discipline) < disciplineRank(keys[j].discipline)
	})

	out := make([]Packet, 0, len(keys))
	for _, k := range keys {
		items := Order(grouped[k])
		packet := Packet{
			Pole: k.pole, Discipline: k.discipline, Items: items,
			NeedsPole: structural[k.pole],
		}
		for _, item := range items {
			if item.Action.PoleWork() {
				packet.NeedsOutage = true
			}
		}
		packet.Owners = ownersOf(items)
		out = append(out, packet)
	}
	return out
}

func disciplineRank(d Discipline) int {
	switch d {
	case DisciplineLine:
		return 0
	case DisciplineComm:
		return 1
	}
	return 2
}

func ownersOf(items []Item) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		name := strings.TrimSpace(item.Owner)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// PacketsFor returns the packets for one crew, in pole order.
func PacketsFor(plan *Plan, d Discipline) []Packet {
	var out []Packet
	for _, p := range Packets(plan) {
		if p.Discipline == d {
			out = append(out, p)
		}
	}
	return out
}

// PolesNeedingAnOutage returns the poles where structure work has to happen
// before anything else on them can, sorted.
func PolesNeedingAnOutage(plan *Plan) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range Packets(plan) {
		if !p.NeedsOutage || seen[p.Pole] {
			continue
		}
		seen[p.Pole] = true
		out = append(out, p.Pole)
	}
	sort.Strings(out)
	return out
}

package cliapp

import (
	"io"

	"github.com/glinskevxer/downguy/internal/makeready"
	"github.com/glinskevxer/downguy/internal/report"
)

func init() {
	Register(Command{
		Name:    "packet",
		Summary: "split a make ready plan into the sheets each crew is handed",
		Usage:   packetUsage(),
		Run:     runPacket,
	})
}

func packetUsage() string {
	f := newPacketFlags(nil)
	return f.Usage("packet [options]", "packet [options]",
		"Reads a make ready plan as json on standard input and splits it into what\n"+
			"one crew is handed on one pole, in the order they do it.\n\n"+
			"The line crew comes before the communication crew on any pole they share,\n"+
			"because everything the communication crew does hangs off what the line crew\n"+
			"leaves. A pole needing structure work is marked as needing an outage, and\n"+
			"the communication packets on it are marked as waiting on it.")
}

func newPacketFlags(out io.Writer) *Flags {
	f := NewFlags("packet", out).WithJSON()
	f.Set().String("crew", "", "one crew only, either line or communication")
	f.Set().Bool("outages", false, "list only the poles that need an outage")
	return f
}

// PacketJSON is one crew sheet on the wire.
type PacketJSON struct {
	Pole        string                `json:"pole"`
	Crew        string                `json:"crew"`
	Items       []report.WorkItemJSON `json:"items"`
	Owners      []string              `json:"owners,omitempty"`
	NeedsPole   bool                  `json:"waits_on_pole_work"`
	NeedsOutage bool                  `json:"needs_outage"`
}

func runPacket(env *Env) int {
	f := newPacketFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "packet reads its plan from standard input and takes no arguments")
	}
	plan, err := ReadPlan(env.Stdin)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	if flagBool(f, "outages") {
		poles := makeready.PolesNeedingAnOutage(plan)
		if f.JSON() {
			if poles == nil {
				poles = []string{}
			}
			if err := report.WriteJSON(env.Stdout, poles); err != nil {
				return env.Fail(CodeInternal, "%v", err)
			}
		} else if len(poles) == 0 {
			env.Println("no pole needs an outage")
		} else {
			for _, pole := range poles {
				env.Println(pole)
			}
		}
		if len(poles) == 0 {
			return CodeOK
		}
		return CodeFindings
	}

	packets := makeready.Packets(plan)
	if crew := stringFlag(f, "crew"); crew != "" {
		discipline := makeready.Discipline(crew)
		if !makeready.KnownDiscipline(discipline) || discipline == makeready.DisciplineUnassigned {
			return env.Fail(CodeUsage, "no crew called %q, try line or communication", crew)
		}
		packets = makeready.PacketsFor(plan, discipline)
	}

	if f.JSON() {
		wire := make([]PacketJSON, 0, len(packets))
		for _, p := range packets {
			row := PacketJSON{
				Pole: p.Pole, Crew: string(p.Discipline), Owners: p.Owners,
				NeedsPole: p.NeedsPole, NeedsOutage: p.NeedsOutage,
				Items: make([]report.WorkItemJSON, 0, len(p.Items)),
			}
			for _, item := range p.Items {
				row.Items = append(row.Items, report.WorkItemToJSON(item))
			}
			wire = append(wire, row)
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else if err := printPackets(env, packets); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if len(packets) == 0 {
		return CodeOK
	}
	return CodeFindings
}

func printPackets(env *Env, packets []makeready.Packet) error {
	if len(packets) == 0 {
		env.Println("no make ready")
		return nil
	}
	for i, p := range packets {
		if i > 0 {
			env.Println()
		}
		env.Println(p)
		if p.NeedsPole && !p.NeedsOutage {
			env.Println("  waits on the structure work on this pole")
		}
		if len(p.Owners) > 0 {
			env.Print("  attachers: %s\n", joinComma(p.Owners))
		}
		table := report.NewTable("item", "action", "subject", "owner", "from", "to", "detail")
		table.RightAlign(4, 5)
		for _, item := range p.Items {
			from, to := "", ""
			if item.FromHeight != 0 || item.ToHeight != 0 {
				from = item.FromHeight.String()
				to = item.ToHeight.String()
			}
			table.Row(item.ID, string(item.Action), item.Subject, item.Owner, from, to, item.Detail)
		}
		if err := table.Render(env.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func joinComma(values []string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

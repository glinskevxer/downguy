package cliapp

import (
	"context"
	"errors"
	"io"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "pole",
		Summary: "show a pole and everything on it",
		Usage:   poleUsage(),
		Run:     runPole,
	})
}

func poleUsage() string {
	f := newPoleFlags(nil)
	return f.Usage("pole <tag>", "pole <tag>",
		"Prints the pole record, the facilities on it from the top down, the anchors,\n"+
			"and the safety zone between supply and communication space.")
}

func newPoleFlags(out io.Writer) *Flags {
	f := NewFlags("pole", out).WithJSON()
	f.Set().Bool("spans", false, "also list the spans landing on the pole")
	return f
}

// PoleJSON is a pole and its contents on the wire.
type PoleJSON struct {
	Pole        string           `json:"pole"`
	Owner       string           `json:"owner"`
	Length      int              `json:"length_ft"`
	Class       string           `json:"class"`
	Species     string           `json:"species"`
	Condition   string           `json:"condition"`
	AboveGrade  string           `json:"above_grade_ft"`
	SafetyZone  string           `json:"safety_zone_ft,omitempty"`
	Attachments []AttachmentJSON `json:"attachments"`
	Anchors     []AnchorJSON     `json:"anchors,omitempty"`
	Spans       []SpanJSON       `json:"spans,omitempty"`
}

// AttachmentJSON is one facility on the wire.
type AttachmentJSON struct {
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Kind       string `json:"kind"`
	Height     string `json:"height_ft"`
	Authorized bool   `json:"authorized"`
	Face       string `json:"face,omitempty"`
}

// AnchorJSON is one anchor on the wire.
type AnchorJSON struct {
	ID      string `json:"id"`
	Lead    int    `json:"lead_ft"`
	Bearing int    `json:"bearing"`
	Guys    int    `json:"guys"`
}

// SpanJSON is one span on the wire.
type SpanJSON struct {
	ID     string `json:"id"`
	Other  string `json:"other_pole"`
	Length int    `json:"length_ft"`
	Cover  string `json:"cover"`
}

func runPole(env *Env) int {
	f := newPoleFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "pole takes exactly one tag")
	}
	id, err := plant.ParsePoleID(rest[0])
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	loader := store.NewPlant(db)
	assembly, err := loader.Assembly(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return env.Fail(CodeFindings, "%v", err)
	}
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	var spans []plant.SpanRecord
	if flagBool(f, "spans") {
		spans, err = loader.Spans().AtPole(ctx, id)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, poleToJSON(assembly, spans)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	return printPole(env, assembly, spans)
}

func poleToJSON(a *plant.Assembly, spans []plant.SpanRecord) PoleJSON {
	out := PoleJSON{
		Pole: string(a.Pole.ID), Owner: a.Pole.Owner, Length: a.Pole.Length,
		Class: string(a.Pole.Class), Species: string(a.Pole.Species),
		Condition: string(a.Pole.Condition), AboveGrade: a.Pole.AboveGrade().String(),
		Attachments: make([]AttachmentJSON, 0, len(a.Attachments)),
	}
	if gap, ok := a.SafetyZone(); ok {
		out.SafetyZone = gap.String()
	}
	for _, att := range a.Attachments {
		out.Attachments = append(out.Attachments, AttachmentJSON{
			ID: att.ID, Owner: att.Owner, Kind: string(att.Kind),
			Height: att.Height.String(), Authorized: att.Authorized, Face: string(att.Face),
		})
	}
	for _, anchor := range a.Anchors {
		out.Anchors = append(out.Anchors, AnchorJSON{
			ID: anchor.ID, Lead: anchor.Lead, Bearing: anchor.Bearing, Guys: len(anchor.Guys),
		})
	}
	for _, s := range spans {
		other, _ := s.Other(a.Pole.ID)
		out.Spans = append(out.Spans, SpanJSON{
			ID: s.ID, Other: string(other), Length: s.Length.Feet(), Cover: string(s.Cover),
		})
	}
	return out
}

func printPole(env *Env, a *plant.Assembly, spans []plant.SpanRecord) int {
	env.Println(a.Pole)
	env.Print("owner        %s\n", a.Pole.Owner)
	env.Print("condition    %s\n", a.Pole.Condition)
	env.Print("above grade  %s\n", a.Pole.AboveGrade())
	if gap, ok := a.SafetyZone(); ok {
		env.Print("safety zone  %s\n", gap)
	}
	env.Println()

	table := report.NewTable("facility", "owner", "kind", "height", "authorized", "face")
	table.RightAlign(3)
	for _, att := range a.Attachments {
		authorized := "no"
		if att.Authorized {
			authorized = "yes"
		}
		table.Row(att.ID, att.Owner, string(att.Kind), att.Height.String(), authorized, string(att.Face))
	}
	if table.Len() == 0 {
		env.Println("nothing on the pole")
	} else if err := table.Render(env.Stdout); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}

	if len(a.Anchors) > 0 {
		env.Println()
		anchors := report.NewTable("anchor", "lead ft", "bearing", "guys")
		anchors.RightAlign(1, 2, 3)
		for _, anchor := range a.Anchors {
			anchors.Row(anchor.ID, itoaCLI(anchor.Lead), itoaCLI(anchor.Bearing), itoaCLI(len(anchor.Guys)))
		}
		if err := anchors.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	if len(spans) > 0 {
		env.Println()
		table := report.NewTable("span", "to", "length ft", "cover")
		table.RightAlign(2)
		for _, s := range spans {
			other, _ := s.Other(a.Pole.ID)
			table.Row(s.ID, string(other), itoaCLI(s.Length.Feet()), string(s.Cover))
		}
		if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	return CodeOK
}

// itoaCLI prints a whole number without dragging strconv into every file here.
func itoaCLI(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return sign + digits
}

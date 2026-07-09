package cliapp

import (
	"context"
	"database/sql"
	"io"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "demo",
		Summary: "load a small worked example into an empty database",
		Usage:   demoUsage(),
		Run:     runDemo,
	})
}

func demoUsage() string {
	f := newDemoFlags(nil)
	return f.Usage("demo [options]", "demo [options]",
		"Loads one line section, four poles, an application and the surveys behind\n"+
			"it, so every other command has something to run against.\n\n"+
			"It refuses a database that already holds poles, because seeding a live\n"+
			"system with a worked example is not a mistake anybody recovers from\n"+
			"quickly. Pass -force to seed one anyway.")
}

func newDemoFlags(out io.Writer) *Flags {
	f := NewFlags("demo", out)
	f.Set().Bool("force", false, "seed a database that already holds records")
	return f
}

func runDemo(env *Env) int {
	f := newDemoFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "demo takes no arguments")
	}
	ctx := context.Background()
	db, cfg, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	loader := store.NewPlant(db)
	existing, err := loader.Poles().Count(ctx)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if existing > 0 && !flagBool(f, "force") {
		return env.Fail(CodeUsage,
			"%s already holds %d poles; pass -force to seed it anyway", cfg.Database, existing)
	}

	if err := seedDemo(ctx, db); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	counts, err := store.Tally(ctx, db)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	env.Print("seeded %s: %d poles, %d attachments, %d spans, %d surveys, %d applications\n",
		cfg.Database, counts.Poles, counts.Attachments, counts.Spans, counts.Surveys, counts.Applications)
	env.Println("try: downguy pole -spans NW-441829")
	return CodeOK
}

// demoPoles is the little run everything else in the example hangs off: a four
// pole section with a road crossing in the middle of it.
func demoPoles() []plant.Pole {
	base := plant.Pole{
		Length: 40, Class: plant.Class4, Species: plant.SouthernPine,
		SetDepth: plant.StandardSetDepth(40), Owner: "SUMMIT VALLEY ELECTRIC",
		Condition: plant.ConditionSound,
	}
	out := make([]plant.Pole, 0, 4)
	for i, tag := range []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"} {
		p := base
		p.ID = tag
		p.InstalledIn = 1996 + i*6
		p.Latitude = 44.0521 + float64(i)*0.0009
		p.Longitude = -123.0868 - float64(i)*0.0011
		out = append(out, p)
	}
	// One old pole to give the inspection programme something to find.
	out[2].InstalledIn = 1968
	out[2].Condition = plant.ConditionSound
	return out
}

func demoAttachments() []plant.Attachment {
	up := calendar.Range{From: calendar.MustNew(2001, 6, 1)}
	cable := calendar.Range{From: calendar.MustNew(2019, 4, 1)}
	fiber := calendar.Range{From: calendar.MustNew(2025, 7, 1)}
	var out []plant.Attachment
	for _, pole := range []plant.PoleID{"NW-441829", "NW-441830", "NW-441831", "NW-441832"} {
		suffix := string(pole)[len(pole)-2:]
		out = append(out,
			plant.Attachment{ID: "PR" + suffix, Pole: pole, Owner: "SUMMIT VALLEY ELECTRIC",
				Kind: plant.KindPrimary, Height: unit.Tenths(315), Authorized: true,
				InForce: up, Voltage: 7620},
			plant.Attachment{ID: "NE" + suffix, Pole: pole, Owner: "SUMMIT VALLEY ELECTRIC",
				Kind: plant.KindNeutral, Height: unit.Tenths(285), Authorized: true, InForce: up},
			plant.Attachment{ID: "CA" + suffix, Pole: pole, Owner: "RIDGEPORT CABLEVISION",
				Kind: plant.KindCommCable, Height: unit.Tenths(200), Authorized: true, InForce: cable},
			plant.Attachment{ID: "FI" + suffix, Pole: pole, Owner: "MERIDIAN FIBER",
				Kind: plant.KindFiber, Height: unit.Tenths(186), Authorized: true, InForce: fiber},
		)
	}
	// One nobody has claimed, for the audit to find.
	out = append(out, plant.Attachment{
		ID: "UN31", Pole: "NW-441831", Owner: "UNIDENTIFIED",
		Kind: plant.KindCommCable, Height: unit.Tenths(174),
		InForce: calendar.Range{From: calendar.MustNew(2021, 3, 1)},
		Note:    "found on a joint use audit, unclaimed",
	})
	return out
}

func demoSpans() []plant.SpanRecord {
	return []plant.SpanRecord{
		{ID: "SP-1", FromPole: "NW-441829", ToPole: "NW-441830",
			Length: unit.SpanFeet(200), Cover: plant.CoverField, Drop: unit.Tenths(-8)},
		{ID: "SP-2", FromPole: "NW-441830", ToPole: "NW-441831",
			Length: unit.SpanFeet(260), Cover: plant.CoverRoad},
		{ID: "SP-3", FromPole: "NW-441831", ToPole: "NW-441832",
			Length: unit.SpanFeet(140), Cover: plant.CoverDriveway},
	}
}

func demoSurveys() []survey.PoleSheet {
	taken := calendar.MustNew(2025, 5, 6)
	var out []survey.PoleSheet
	for _, pole := range []plant.PoleID{"NW-441829", "NW-441830", "NW-441831"} {
		suffix := string(pole)[len(pole)-2:]
		sheet := survey.PoleSheet{
			ID: "SV-" + suffix, Pole: pole, Taken: taken, Crew: "CREW 41",
			Condition: plant.ConditionSound,
			Measurements: []survey.Measurement{
				{Subject: "PR" + suffix, Height: unit.Tenths(315), Method: survey.MethodLaser, Crew: "CREW 41"},
				{Subject: "NE" + suffix, Height: unit.Tenths(285), Method: survey.MethodLaser, Crew: "CREW 41"},
				{Subject: "CA" + suffix, Height: unit.Tenths(200), Method: survey.MethodStick, Crew: "CREW 41"},
				{Subject: "FI" + suffix, Height: unit.Tenths(186), Method: survey.MethodStick, Crew: "CREW 41"},
			},
		}
		if pole == "NW-441831" {
			sheet.Condition = plant.ConditionServiceabe
			sheet.Measurements = append(sheet.Measurements, survey.Measurement{
				Subject: "UN31", Height: unit.Tenths(174), Method: survey.MethodStick, Crew: "CREW 41",
			})
		}
		out = append(out, sheet)
	}
	return out
}

// seedDemo writes the example. Everything goes through the ordinary
// repositories, so a record the example cannot store is a record the system
// would have refused anyway.
func seedDemo(ctx context.Context, db *sql.DB) error {
	loader := store.NewPlant(db)
	for _, pole := range demoPoles() {
		if err := loader.Poles().Put(ctx, pole); err != nil {
			return err
		}
	}
	if err := loader.Attachments().PutAll(ctx, demoAttachments()); err != nil {
		return err
	}
	for _, span := range demoSpans() {
		if err := loader.Spans().Put(ctx, span); err != nil {
			return err
		}
	}
	surveys := store.NewSurveys(db)
	for _, sheet := range demoSurveys() {
		if err := surveys.Put(ctx, sheet); err != nil {
			return err
		}
	}

	app, err := permit.New("APP-1042", "MERIDIAN FIBER", "SUMMIT VALLEY ELECTRIC",
		[]plant.PoleID{"NW-441829", "NW-441830", "NW-441831"},
		calendar.MustNew(2025, 3, 3), "j.aldana")
	if err != nil {
		return err
	}
	for _, step := range []struct {
		to permit.State
		on calendar.Date
	}{
		{permit.StateSubmitted, calendar.MustNew(2025, 3, 5)},
		{permit.StateAccepted, calendar.MustNew(2025, 3, 12)},
		{permit.StateSurveying, calendar.MustNew(2025, 3, 20)},
	} {
		if err := app.MoveTo(step.to, step.on, "d.orme", ""); err != nil {
			return err
		}
	}
	if err := store.NewApplications(db).Put(ctx, app); err != nil {
		return err
	}
	_, err = store.NewChangeLog(db).Append(ctx, store.Change{
		At: calendar.MustNew(2025, 5, 6), Subject: store.SubjectPole, ID: "demo",
		Action: store.ActionImported, Actor: "demo", Detail: "worked example seeded",
	})
	return err
}

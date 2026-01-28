package cliapp

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/rental"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "bill",
		Summary: "run the annual rental billing for a rate year",
		Usage:   billUsage(),
		Run:     runBill,
	})
	Register(Command{
		Name:    "audit",
		Summary: "find unauthorized attachments and work out what they owe",
		Usage:   auditUsage(),
		Run:     runAudit,
	})
}

func billUsage() string {
	f := newBillFlags(nil)
	return f.Usage("bill <year> [options]", "bill <year> [options]",
		"Builds the billable inventory for a rate year out of the plant records,\n"+
			"works out the rate on every pole from how many attachers are actually on\n"+
			"it, prorates anything that was not up all year, and raises one bill per\n"+
			"attacher.\n\n"+
			"Nothing is written unless -commit is given, so the run can be checked\n"+
			"before it becomes an invoice.")
}

func newBillFlags(out io.Writer) *Flags {
	f := NewFlags("bill", out).WithJSON()
	f.Set().String("issued", "", "the date the bills are issued, as yyyy-mm-dd")
	f.Set().String("prefix", "RB", "the prefix for the bill numbers")
	f.Set().String("telecom", "", "comma separated attachers billed on the telecom formula")
	f.Set().Bool("commit", false, "write the bills to the database")
	f.Set().Bool("csv", false, "write the run as csv, one row per attachment")
	return f
}

func runBill(env *Env) int {
	f := newBillFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "bill takes exactly one year")
	}
	year := 0
	if _, err := fmt.Sscanf(rest[0], "%d", &year); err != nil || year < 1900 {
		return env.Fail(CodeUsage, "%q is not a rate year", rest[0])
	}
	issued, err := requiredDateFlag(f, "issued")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	prefix := f.Set().Lookup("prefix").Value.String()

	assignment := rental.Assignment{}
	for _, name := range strings.Split(f.Set().Lookup("telecom").Value.String(), ",") {
		name = strings.ToUpper(strings.TrimSpace(name))
		if name != "" {
			assignment[name] = rental.FormulaTelecom
		}
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	inventory, err := buildInventory(ctx, db, calendar.RateYear(year))
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	bills, err := rental.Bills(inventory, assignment, issued, prefix)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	if flagBool(f, "commit") {
		if err := store.NewBilling(db).SaveBills(ctx, bills); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	switch {
	case flagBool(f, "csv"):
		if err := ingest.WriteBills(env.Stdout, bills); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	case f.JSON():
		if err := report.WriteJSON(env.Stdout, billsToJSON(bills)); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	default:
		if err := printBills(env, bills, inventory); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	if len(bills) == 0 {
		return CodeFindings
	}
	return CodeOK
}

// BillJSON is one attacher's rental on the wire.
type BillJSON struct {
	Number   string `json:"number"`
	Year     int    `json:"year"`
	Owner    string `json:"owner"`
	Formula  string `json:"formula"`
	Count    int    `json:"attachments"`
	Prorated int    `json:"prorated"`
	Flagged  int    `json:"flagged"`
	Total    string `json:"total"`
	Issued   string `json:"issued"`
	Due      string `json:"due"`
}

func billsToJSON(bills []rental.Bill) []BillJSON {
	out := make([]BillJSON, 0, len(bills))
	for _, b := range bills {
		out = append(out, BillJSON{
			Number: b.Number, Year: int(b.Year), Owner: b.Owner, Formula: string(b.Formula),
			Count: len(b.Lines), Prorated: len(b.Prorated()), Flagged: len(b.Flagged()),
			Total: b.Total.String(), Issued: b.Issued.String(), Due: b.Due.String(),
		})
	}
	return out
}

func printBills(env *Env, bills []rental.Bill, inventory *rental.Inventory) error {
	env.Print("%s\n\n", inventory)
	table := report.NewTable("number", "attacher", "formula", "attachments", "prorated", "flagged", "total", "due")
	table.RightAlign(3, 4, 5, 6)
	for _, b := range bills {
		table.Row(b.Number, b.Owner, string(b.Formula), itoaCLI(len(b.Lines)),
			itoaCLI(len(b.Prorated())), itoaCLI(len(b.Flagged())), b.Total.String(), b.Due.String())
	}
	if err := table.Render(env.Stdout); err != nil {
		return err
	}
	env.Print("\nbilled %s across %d attachers\n", rental.BillTotal(bills), len(bills))
	return nil
}

func auditUsage() string {
	f := newAuditFlags(nil)
	return f.Usage("audit <year> [options]", "audit <year> [options]",
		"Finds every attachment on file that nobody has permission for and works out\n"+
			"the back rent and the penalty under the joint use agreement.\n\n"+
			"Exit code 1 means there are claims.")
}

func newAuditFlags(out io.Writer) *Flags {
	f := NewFlags("audit", out).WithJSON()
	f.Set().String("found", "", "the date the attachments were found, as yyyy-mm-dd")
	f.Set().String("noticed", "", "the date the audit notice went out, as yyyy-mm-dd")
	f.Set().Bool("self-reported", false, "treat the attachments as self reported rather than found")
	return f
}

// ClaimJSON is one unauthorized attachment claim on the wire.
type ClaimJSON struct {
	Attachment string `json:"attachment"`
	Pole       string `json:"pole"`
	Owner      string `json:"owner"`
	Sanction   string `json:"sanction"`
	BackYears  int    `json:"back_years"`
	BackRent   string `json:"back_rent"`
	Penalty    string `json:"penalty"`
	Total      string `json:"total"`
	Note       string `json:"note,omitempty"`
}

func runAudit(env *Env) int {
	f := newAuditFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "audit takes exactly one year")
	}
	year := 0
	if _, err := fmt.Sscanf(rest[0], "%d", &year); err != nil || year < 1900 {
		return env.Fail(CodeUsage, "%q is not a rate year", rest[0])
	}
	found, err := requiredDateFlag(f, "found")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	noticed, err := dateFlag(f, "noticed")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	discovery := rental.DiscoveryAudit
	if flagBool(f, "self-reported") {
		discovery = rental.DiscoverySelfReported
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	inventory, err := buildInventory(ctx, db, calendar.RateYear(year))
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	claims, err := rental.AssessAll(inventory, discovery, found, noticed, nil)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	if f.JSON() {
		wire := make([]ClaimJSON, 0, len(claims))
		for _, c := range claims {
			wire = append(wire, ClaimJSON{
				Attachment: c.AttachmentID, Pole: string(c.Pole), Owner: c.Owner,
				Sanction: string(c.Sanction), BackYears: len(c.BackYears),
				BackRent: c.BackRent.String(), Penalty: c.Penalty.String(),
				Total: c.Total.String(), Note: c.Note,
			})
		}
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		table := report.NewTable("attachment", "pole", "owner", "sanction", "years", "back rent", "penalty", "total")
		table.RightAlign(4, 5, 6, 7)
		for _, c := range claims {
			table.Row(c.AttachmentID, string(c.Pole), c.Owner, string(c.Sanction),
				itoaCLI(len(c.BackYears)), c.BackRent.String(), c.Penalty.String(), c.Total.String())
		}
		if table.Len() == 0 {
			env.Println("no unauthorized attachments")
		} else if err := table.Render(env.Stdout); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		if len(claims) > 0 {
			env.Print("\nclaimed %s across %d attachments\n", rental.ClaimTotal(claims), len(claims))
		}
	}
	if len(claims) == 0 {
		return CodeOK
	}
	return CodeFindings
}

// buildInventory walks every pole in the database and files the communication
// attachments on it into a rate year inventory. It is the one place that turns
// stored plant into something billable, so both the billing run and the audit
// see exactly the same set.
func buildInventory(ctx context.Context, db *sql.DB, year calendar.RateYear) (*rental.Inventory, error) {
	loader := store.NewPlant(db)
	poles, err := loader.Poles().All(ctx)
	if err != nil {
		return nil, err
	}
	inventory := rental.NewInventory(year)
	for _, pole := range poles {
		assembly, err := loader.Assembly(ctx, pole.ID)
		if err != nil {
			return nil, err
		}
		if err := inventory.FromAssembly(assembly); err != nil {
			return nil, err
		}
	}
	return inventory, nil
}

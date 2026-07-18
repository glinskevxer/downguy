package cliapp

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/glinskevxer/downguy/internal/allocate"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/costing"
	"github.com/glinskevxer/downguy/internal/report"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() {
	Register(Command{
		Name:    "allocate",
		Summary: "decide who pays for the make ready and raise the invoices",
		Usage:   allocateUsage(),
		Run:     runAllocate,
	})
	Register(Command{
		Name:    "policies",
		Summary: "list the cost allocation policies",
		Usage:   policiesUsage(),
		Run:     runPolicies,
	})
}

func allocateUsage() string {
	f := newAllocateFlags(nil)
	return f.Usage("allocate [options]", "allocate [options]",
		"Reads a make ready plan as json on standard input, prices it, and lands\n"+
			"every priced item on a party under the allocation policy.\n\n"+
			"The allocation keys on the attributed findings, so -findings takes the\n"+
			"json the makeready command writes: that document carries them. The plain\n"+
			"clearance output will not do, because its findings are unattributed and an\n"+
			"unattributed finding cannot be billed to anybody.\n\n"+
			"The parties come from -owner, -applicant and repeated -attacher options,\n"+
			"in the form name=account.")
}

func newAllocateFlags(out io.Writer) *Flags {
	f := NewFlags("allocate", out).WithJSON()
	f.Set().String("on", "", "the date to price against, as yyyy-mm-dd")
	f.Set().String("policy", "", "the allocation policy, overriding the configuration")
	f.Set().String("owner", "", "the pole owner, as name=account")
	f.Set().String("applicant", "", "the applicant, as name=account")
	f.Set().Var(&partyFlag{}, "attacher", "an existing attacher, as name=account")
	f.Set().String("findings", "", "path to the attributed findings json")
	f.Set().String("prefix", "MRI", "the prefix for the invoice numbers")
	return f
}

// partyFlag collects repeated existing attachers off the command line.
type partyFlag struct{ pairs []string }

func (p *partyFlag) String() string { return strings.Join(p.pairs, ",") }

func (p *partyFlag) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("a party looks like name=account")
	}
	p.pairs = append(p.pairs, value)
	return nil
}

func splitParty(value string) (string, string, bool) {
	at := strings.Index(value, "=")
	if at < 0 {
		return "", "", false
	}
	name := strings.TrimSpace(value[:at])
	account := strings.TrimSpace(value[at+1:])
	if name == "" || account == "" {
		return "", "", false
	}
	return name, account, true
}

// SplitJSON is an allocation on the wire.
type SplitJSON struct {
	Application string        `json:"application"`
	Policy      string        `json:"policy"`
	Charges     []ChargeJSON  `json:"charges"`
	Unallocated []string      `json:"unallocated,omitempty"`
	Total       string        `json:"total"`
	Invoices    []InvoiceJSON `json:"invoices"`
}

// ChargeJSON is one priced item landed on a party.
type ChargeJSON struct {
	Item   string `json:"item"`
	Pole   string `json:"pole"`
	Payer  string `json:"payer"`
	Party  string `json:"party"`
	Blame  string `json:"blame"`
	Amount string `json:"amount"`
}

// InvoiceJSON is one party's invoice.
type InvoiceJSON struct {
	Number string `json:"number"`
	Party  string `json:"party"`
	Role   string `json:"role"`
	Lines  int    `json:"lines"`
	Total  string `json:"total"`
	Due    string `json:"due"`
}

func runAllocate(env *Env) int {
	f := newAllocateFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "allocate reads its plan from standard input and takes no arguments")
	}
	on, err := requiredDateFlag(f, "on")
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	cfg, err := f.Config(env)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	policy, err := cfg.AllocationPolicy()
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	if override := f.Set().Lookup("policy").Value.String(); override != "" {
		if policy, err = allocate.PolicyNamed(override); err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
	}

	roster, err := rosterFrom(f)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	findings, err := readFindings(f.Set().Lookup("findings").Value.String())
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	plan, err := ReadPlan(env.Stdin)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	estimate, err := costing.Price(plan, costing.Standard(), on)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	split, err := allocate.Allocate(plan.Application, policy, roster, plan, estimate, findings)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}
	invoices, err := allocate.Invoices(split, roster, on, f.Set().Lookup("prefix").Value.String())
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	wire := SplitJSON{
		Application: split.Application, Policy: split.Policy.Name,
		Charges:     make([]ChargeJSON, 0, len(split.Charges)),
		Unallocated: split.Unallocated, Total: split.Total.String(),
		Invoices: make([]InvoiceJSON, 0, len(invoices)),
	}
	for _, c := range split.Charges {
		wire.Charges = append(wire.Charges, ChargeJSON{
			Item: c.ItemID, Pole: c.Pole, Payer: string(c.Payer),
			Party: c.Party, Blame: string(c.Blame), Amount: c.Amount.String(),
		})
	}
	for _, i := range invoices {
		wire.Invoices = append(wire.Invoices, InvoiceJSON{
			Number: i.Number, Party: i.Party, Role: string(i.Role),
			Lines: len(i.Lines), Total: i.Total.String(), Due: i.Due.String(),
		})
	}

	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, wire); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	} else {
		env.Print("%s\n\n", split)
		charges := report.NewTable("item", "pole", "payer", "party", "blame", "amount")
		charges.RightAlign(5)
		for _, c := range wire.Charges {
			charges.Row(c.Item, c.Pole, c.Payer, c.Party, c.Blame, c.Amount)
		}
		if charges.Len() > 0 {
			if err := charges.Render(env.Stdout); err != nil {
				return env.Fail(CodeInternal, "%v", err)
			}
		}
		totals := map[string]string{}
		for party, amount := range split.ByParty() {
			totals[party] = amount.String()
		}
		env.Println()
		if err := report.OwnerTotals(env.Stdout, "invoiced by party", totals); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}
	for _, id := range split.Unallocated {
		fmt.Fprintf(env.Stderr, "nothing to bill %s against\n", id)
	}
	if len(split.Unallocated) > 0 {
		return CodeFindings
	}
	return CodeOK
}

// rosterFrom builds the party roster out of the command line.
func rosterFrom(f *Flags) (*allocate.Roster, error) {
	roster := allocate.NewRoster()
	for _, pair := range []struct {
		flag string
		role allocate.Role
	}{
		{"owner", allocate.RoleOwner},
		{"applicant", allocate.RoleApplicant},
	} {
		value := f.Set().Lookup(pair.flag).Value.String()
		if value == "" {
			return nil, fmt.Errorf("-%s is required and looks like name=account", pair.flag)
		}
		name, account, ok := splitParty(value)
		if !ok {
			return nil, fmt.Errorf("-%s looks like name=account", pair.flag)
		}
		if err := roster.Add(allocate.Party{Name: name, Role: pair.role, Account: account}); err != nil {
			return nil, err
		}
	}
	if value := f.Set().Lookup("attacher"); value != nil {
		if typed, ok := value.Value.(*partyFlag); ok {
			for _, entry := range typed.pairs {
				name, account, ok := splitParty(entry)
				if !ok {
					return nil, fmt.Errorf("-attacher looks like name=account")
				}
				err := roster.Add(allocate.Party{Name: name, Role: allocate.RoleExisting, Account: account})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return roster, roster.Validate()
}

// readFindings loads the attributed findings the allocation is keyed on.
func readFindings(path string) (clearance.Findings, error) {
	if path == "" {
		return nil, fmt.Errorf("-findings is required and takes the json the makeready command writes")
	}
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var wire struct {
		Unresolved []report.FindingJSON `json:"unresolved"`
		Findings   []report.FindingJSON `json:"findings"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		var bare []report.FindingJSON
		if err := json.Unmarshal(body, &bare); err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		wire.Findings = bare
	}
	rows := wire.Findings
	if len(rows) == 0 {
		rows = wire.Unresolved
	}
	out := make(clearance.Findings, 0, len(rows))
	for _, r := range rows {
		measured, err := unit.ParseHeight(orZero(r.Measured))
		if err != nil {
			return nil, fmt.Errorf("%s: measured_ft: %w", r.Rule, err)
		}
		required, err := unit.ParseHeight(orZero(r.Required))
		if err != nil {
			return nil, fmt.Errorf("%s: required_ft: %w", r.Rule, err)
		}
		out = append(out, clearance.Finding{
			RuleID: r.Rule, Pole: r.Pole, Span: r.Span, Subject: r.Subject,
			Against: r.Against, Owner: r.Owner, Blame: clearance.Blame(r.Blame),
			Measured: measured, Required: required, Detail: r.Detail,
		})
	}
	return out, nil
}

func orZero(text string) string {
	if strings.TrimSpace(text) == "" {
		return "0"
	}
	return text
}

func policiesUsage() string {
	f := NewFlags("policies", nil).WithJSON()
	return f.Usage("policies [options]", "policies [options]",
		"Lists the cost allocation policies, with who each one bills for a pre\n"+
			"existing condition.")
}

func runPolicies(env *Env) int {
	f := NewFlags("policies", env.Stderr).WithJSON()
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	if len(f.Rest()) > 0 {
		return env.Fail(CodeUsage, "policies takes no arguments")
	}
	type row struct {
		Name             string `json:"name"`
		PaysCaused       bool   `json:"applicant_pays_caused"`
		PaysAggravated   bool   `json:"applicant_pays_aggravated"`
		PreExistingPayer string `json:"pre_existing_payer"`
		ApplicantShare   string `json:"applicant_share,omitempty"`
	}
	var rows []row
	for _, p := range allocate.Policies() {
		r := row{
			Name: p.Name, PaysCaused: p.ApplicantPaysCaused,
			PaysAggravated:   p.ApplicantPaysAggravated,
			PreExistingPayer: string(p.PreExistingPayer),
		}
		if p.PreExistingPayer == allocate.PayerShared {
			r.ApplicantShare = p.ApplicantShare.String()
		}
		rows = append(rows, r)
	}
	if f.JSON() {
		if err := report.WriteJSON(env.Stdout, rows); err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		return CodeOK
	}
	table := report.NewTable("policy", "caused", "aggravated", "pre existing", "share")
	for _, r := range rows {
		table.Row(r.Name, yesNo(r.PaysCaused), yesNo(r.PaysAggravated), r.PreExistingPayer, r.ApplicantShare)
	}
	return renderOrFail(env, table)
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

package apiserver

import (
	"net/http"
	"strconv"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/rental"
)

func init() { registerRateRoutes() }

var rateRoutes []func(*Server)

func registerRateRoutes() {
	rateRoutes = append(rateRoutes,
		func(s *Server) { s.route("/rates/", s.handleRate) },
		func(s *Server) { s.route("/rules", s.handleRules) },
	)
}

// RegisterRates adds the published rate and code rule endpoints. They are the
// two things an attacher portal asks for that are not about one pole, and they
// read nothing from the database at all, which is why they can be served even
// when the plant tables are empty.
func (s *Server) RegisterRates() *Server {
	for _, add := range rateRoutes {
		add(s)
	}
	return s
}

// RateJSON is one worked rate on the wire.
type RateJSON struct {
	Year       int    `json:"year"`
	Formula    string `json:"formula"`
	Attachers  int    `json:"attachers"`
	SpaceShare string `json:"space_factor"`
	PoleCost   string `json:"annual_pole_cost"`
	Annual     string `json:"annual_rate"`
	Monthly    string `json:"monthly_rate"`
}

// RateYearJSON is a whole published year.
type RateYearJSON struct {
	Year      int        `json:"year"`
	Attachers int        `json:"attachers"`
	Rates     []RateJSON `json:"rates"`
}

func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	param, err := PathParam(r.URL.Path, "/rates/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	year, err := strconv.Atoi(param)
	if err != nil || year < 1900 {
		writeError(w, http.StatusBadRequest, "the path names a rate year")
		return
	}
	attachers := s.cfg.Attachers
	if raw := r.URL.Query().Get("attachers"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 20 {
			writeError(w, http.StatusBadRequest, "attachers takes a whole number from one to twenty")
			return
		}
		attachers = parsed
	}

	out := RateYearJSON{Year: year, Attachers: attachers}
	for _, formula := range rental.Formulas() {
		rate, err := rental.Standard(calendar.RateYear(year), formula, attachers)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		out.Rates = append(out.Rates, RateJSON{
			Year: year, Formula: string(formula), Attachers: attachers,
			SpaceShare: rate.Inputs.SpaceShare.String(),
			PoleCost:   rate.Inputs.PoleCost.String(),
			Annual:     rate.Annual.String(), Monthly: rate.Monthly().String(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// RuleJSON is one code rule on the wire.
type RuleJSON struct {
	ID       string `json:"id"`
	Section  string `json:"section"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Applies  string `json:"applies_to"`
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")
	var rules []nesc.Rule
	if section != "" {
		rules = nesc.RulesInSection(section)
		if len(rules) == 0 {
			writeError(w, http.StatusNotFound, "no rules in section "+section)
			return
		}
	} else {
		for _, id := range nesc.RuleIDs() {
			rule, err := nesc.LookupRule(id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			rules = append(rules, rule)
		}
	}
	out := make([]RuleJSON, 0, len(rules))
	for _, rule := range rules {
		out = append(out, RuleJSON{
			ID: rule.ID, Section: rule.Section, Title: rule.Title,
			Severity: string(rule.Severity), Applies: rule.Applies,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

package apiserver

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/clearance"
	"github.com/glinskevxer/downguy/internal/nesc"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

func init() { registerFindingRoutes() }

var findingRoutes []func(*Server)

func registerFindingRoutes() {
	findingRoutes = append(findingRoutes,
		func(s *Server) { s.route("/findings/", s.handleFindings) },
	)
}

// RegisterFindings adds the live clearance endpoint. It runs the rules on
// request rather than reading a stored result, because a stored result is only
// as fresh as the last sweep and the question a portal asks is about the pole
// as it stands now.
func (s *Server) RegisterFindings() *Server {
	for _, add := range findingRoutes {
		add(s)
	}
	return s
}

// FindingJSON is one clearance finding on the wire.
type FindingJSON struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Pole     string `json:"pole,omitempty"`
	Subject  string `json:"subject"`
	Against  string `json:"against,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Measured string `json:"measured_ft"`
	Required string `json:"required_ft"`
	Deficit  string `json:"deficit_ft"`
	Detail   string `json:"detail,omitempty"`
}

// FindingsJSON is what a live check on one pole returns.
type FindingsJSON struct {
	Pole     string        `json:"pole"`
	Surveyed string        `json:"surveyed"`
	District string        `json:"district"`
	Grade    string        `json:"grade"`
	Stale    bool          `json:"survey_is_stale"`
	Blocking bool          `json:"blocking"`
	Findings []FindingJSON `json:"findings"`
}

func (s *Server) handleFindings(w http.ResponseWriter, r *http.Request) {
	tag, err := PathParam(r.URL.Path, "/findings/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := plant.ParsePoleID(tag)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	district, err := s.cfg.LoadingDistrict()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if raw := r.URL.Query().Get("district"); raw != "" {
		district, err = nesc.ParseDistrict(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	grade, err := s.cfg.ConstructionGrade()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if raw := r.URL.Query().Get("grade"); raw != "" {
		grade, err = nesc.ParseGrade(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	ctx := r.Context()
	assembly, err := s.plant.Assembly(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sheet, err := store.NewSurveys(s.db).Latest(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		// A pole nobody has been out to is not a pole with no findings, and
		// the difference is the whole point of saying so with a status rather
		// than an empty list.
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	applied, err := survey.Apply(sheet, assembly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	found, err := clearance.New().CheckPole(clearance.PoleContext{
		Applied: applied, District: district, Grade: grade,
		Applicant: r.URL.Query().Get("applicant"),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	out := FindingsJSON{
		Pole: string(id), Surveyed: sheet.Taken.String(),
		District: string(district), Grade: string(grade),
		Blocking: found.Blocking(),
		Findings: make([]FindingJSON, 0, len(found)),
	}
	if asOf := r.URL.Query().Get("as_of"); asOf != "" {
		day, err := parseQueryDate(asOf)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		out.Stale = sheet.Stale(day)
	}
	for _, f := range found {
		out.Findings = append(out.Findings, FindingJSON{
			Rule: f.RuleID, Severity: string(f.Severity), Pole: f.Pole,
			Subject: f.Subject, Against: f.Against, Owner: f.Owner,
			Measured: unit.Decimal(f.Measured),
			Required: unit.Decimal(f.Required),
			Deficit:  unit.Decimal(f.Deficit()),
			Detail:   f.Detail,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// parseQueryDate reads a date out of a query string, with the parameter named
// in the error so a caller knows which one it got wrong.
func parseQueryDate(text string) (calendar.Date, error) {
	day, err := calendar.ParseDate(text)
	if err != nil {
		return calendar.Zero, fmt.Errorf("as_of: %w", err)
	}
	return day, nil
}

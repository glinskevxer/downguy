package apiserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() { registerPlantRoutes() }

// plantRoutes is the set of handlers added by this file. They are registered
// on a server by RegisterPlant rather than in New, so a caller can stand up a
// server with only the probes on it.
var plantRoutes []func(*Server)

func registerPlantRoutes() {
	plantRoutes = append(plantRoutes,
		func(s *Server) { s.route("/poles", s.handlePoleIndex) },
		func(s *Server) { s.route("/poles/", s.handlePole) },
		func(s *Server) { s.route("/spans/", s.handleSpan) },
		func(s *Server) { s.route("/attachers/", s.handleAttacher) },
	)
}

// RegisterPlant adds the plant endpoints to a server.
func (s *Server) RegisterPlant() *Server {
	for _, add := range plantRoutes {
		add(s)
	}
	return s
}

// PoleSummaryJSON is a pole in a listing.
type PoleSummaryJSON struct {
	ID          string `json:"id"`
	District    string `json:"district"`
	Owner       string `json:"owner"`
	Length      int    `json:"length_ft"`
	Class       string `json:"class"`
	Species     string `json:"species"`
	Condition   string `json:"condition"`
	Attachments int    `json:"attachments"`
}

// PoleIndexJSON is a page of poles.
type PoleIndexJSON struct {
	Count int               `json:"count"`
	Poles []PoleSummaryJSON `json:"poles"`
}

func (s *Server) handlePoleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	var (
		poles []plant.Pole
		err   error
	)
	switch {
	case query.Get("district") != "":
		poles, err = s.plant.Poles().InDistrict(ctx, strings.ToUpper(query.Get("district")))
	case query.Get("owner") != "":
		poles, err = s.plant.Poles().OwnedBy(ctx, query.Get("owner"))
	default:
		poles, err = s.plant.Poles().All(ctx)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit := len(poles)
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "limit takes a whole number of at least one")
			return
		}
		if parsed < limit {
			limit = parsed
		}
	}

	out := PoleIndexJSON{Count: len(poles), Poles: make([]PoleSummaryJSON, 0, limit)}
	for _, pole := range poles[:limit] {
		count, err := s.plant.Attachments().CountOnPole(ctx, pole.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out.Poles = append(out.Poles, PoleSummaryJSON{
			ID: string(pole.ID), District: pole.ID.District(), Owner: pole.Owner,
			Length: pole.Length, Class: string(pole.Class), Species: string(pole.Species),
			Condition: string(pole.Condition), Attachments: count,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// PoleDetailJSON is one pole with everything on it.
type PoleDetailJSON struct {
	PoleSummaryJSON
	AboveGrade string              `json:"above_grade_ft"`
	SafetyZone string              `json:"safety_zone_ft,omitempty"`
	Facilities []FacilityJSON      `json:"facilities"`
	Anchors    []AnchorSummaryJSON `json:"anchors,omitempty"`
	Spans      []SpanSummaryJSON   `json:"spans,omitempty"`
}

// FacilityJSON is one attachment on the wire.
type FacilityJSON struct {
	ID         string `json:"id"`
	Owner      string `json:"owner"`
	Kind       string `json:"kind"`
	Height     string `json:"height_ft"`
	Authorized bool   `json:"authorized"`
	Face       string `json:"face,omitempty"`
}

// AnchorSummaryJSON is one anchor on the wire.
type AnchorSummaryJSON struct {
	ID      string `json:"id"`
	Lead    int    `json:"lead_ft"`
	Bearing int    `json:"bearing"`
	Guys    int    `json:"guys"`
}

// SpanSummaryJSON is one span on the wire.
type SpanSummaryJSON struct {
	ID     string `json:"id"`
	Other  string `json:"other_pole"`
	Length int    `json:"length_ft"`
	Cover  string `json:"cover"`
}

func (s *Server) handlePole(w http.ResponseWriter, r *http.Request) {
	tag, err := PathParam(r.URL.Path, "/poles/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := plant.ParsePoleID(tag)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
	spans, err := s.plant.Spans().AtPole(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := PoleDetailJSON{
		PoleSummaryJSON: PoleSummaryJSON{
			ID: string(assembly.Pole.ID), District: assembly.Pole.ID.District(),
			Owner: assembly.Pole.Owner, Length: assembly.Pole.Length,
			Class: string(assembly.Pole.Class), Species: string(assembly.Pole.Species),
			Condition: string(assembly.Pole.Condition), Attachments: len(assembly.Attachments),
		},
		AboveGrade: assembly.Pole.AboveGrade().String(),
		Facilities: make([]FacilityJSON, 0, len(assembly.Attachments)),
	}
	if gap, ok := assembly.SafetyZone(); ok {
		out.SafetyZone = gap.String()
	}
	for _, att := range assembly.Attachments {
		out.Facilities = append(out.Facilities, FacilityJSON{
			ID: att.ID, Owner: att.Owner, Kind: string(att.Kind),
			Height: att.Height.String(), Authorized: att.Authorized, Face: string(att.Face),
		})
	}
	for _, anchor := range assembly.Anchors {
		out.Anchors = append(out.Anchors, AnchorSummaryJSON{
			ID: anchor.ID, Lead: anchor.Lead, Bearing: anchor.Bearing, Guys: len(anchor.Guys),
		})
	}
	for _, span := range spans {
		other, _ := span.Other(id)
		out.Spans = append(out.Spans, SpanSummaryJSON{
			ID: span.ID, Other: string(other), Length: span.Length.Feet(), Cover: string(span.Cover),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// SpanDetailJSON is one span on the wire.
type SpanDetailJSON struct {
	ID       string `json:"id"`
	FromPole string `json:"from_pole"`
	ToPole   string `json:"to_pole"`
	Length   int    `json:"length_ft"`
	Cover    string `json:"cover"`
	Drop     string `json:"drop_ft"`
	DeadEnd  bool   `json:"dead_end"`
	Level    bool   `json:"level"`
}

func (s *Server) handleSpan(w http.ResponseWriter, r *http.Request) {
	id, err := PathParam(r.URL.Path, "/spans/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	span, err := s.plant.Spans().Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SpanDetailJSON{
		ID: span.ID, FromPole: string(span.FromPole), ToPole: string(span.ToPole),
		Length: span.Length.Feet(), Cover: string(span.Cover),
		Drop: span.Drop.String(), DeadEnd: span.Dead, Level: span.Level(),
	})
}

// AttacherJSON is everything one attacher has on the system.
type AttacherJSON struct {
	Owner        string         `json:"owner"`
	Count        int            `json:"count"`
	Unauthorized int            `json:"unauthorized"`
	Poles        []string       `json:"poles"`
	Facilities   []FacilityJSON `json:"facilities"`
}

func (s *Server) handleAttacher(w http.ResponseWriter, r *http.Request) {
	name, err := PathParam(r.URL.Path, "/attachers/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	atts, err := s.plant.Attachments().OwnedBy(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(atts) == 0 {
		writeError(w, http.StatusNotFound, "nothing on file for "+name)
		return
	}
	out := AttacherJSON{Owner: name, Count: len(atts),
		Facilities: make([]FacilityJSON, 0, len(atts))}
	seen := map[string]bool{}
	for _, att := range atts {
		if !att.Authorized {
			out.Unauthorized++
		}
		if !seen[string(att.Pole)] {
			seen[string(att.Pole)] = true
			out.Poles = append(out.Poles, string(att.Pole))
		}
		out.Facilities = append(out.Facilities, FacilityJSON{
			ID: att.ID, Owner: att.Owner, Kind: string(att.Kind),
			Height: att.Height.String(), Authorized: att.Authorized, Face: string(att.Face),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

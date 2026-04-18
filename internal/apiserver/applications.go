package apiserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/glinskevxer/downguy/internal/permit"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() { registerApplicationRoutes() }

var applicationRoutes []func(*Server)

func registerApplicationRoutes() {
	applicationRoutes = append(applicationRoutes,
		func(s *Server) { s.route("/applications", s.handleApplicationIndex) },
		func(s *Server) { s.route("/applications/", s.handleApplication) },
	)
}

// RegisterApplications adds the application endpoints. They are what an
// attacher portal reads: where its own applications have got to, who is
// holding each one, and how long it has been sitting there.
func (s *Server) RegisterApplications() *Server {
	for _, add := range applicationRoutes {
		add(s)
	}
	return s
}

// ApplicationSummaryJSON is one application in a listing.
type ApplicationSummaryJSON struct {
	ID        string `json:"id"`
	Applicant string `json:"applicant"`
	Owner     string `json:"owner"`
	State     string `json:"state"`
	HeldBy    string `json:"held_by"`
	Live      bool   `json:"live"`
	Opened    string `json:"opened"`
	Poles     int    `json:"poles"`
}

// ApplicationIndexJSON is a page of applications.
type ApplicationIndexJSON struct {
	Count        int                      `json:"count"`
	Applications []ApplicationSummaryJSON `json:"applications"`
}

func (s *Server) handleApplicationIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	repo := store.NewApplications(s.db)
	query := r.URL.Query()

	var (
		apps []*permit.Application
		err  error
	)
	switch {
	case query.Get("state") != "":
		state, parseErr := permit.ParseState(query.Get("state"))
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, parseErr.Error())
			return
		}
		apps, err = repo.InState(ctx, state)
	case query.Get("applicant") != "":
		apps, err = repo.ForApplicant(ctx, query.Get("applicant"))
	case query.Get("pole") != "":
		id, parseErr := plant.ParsePoleID(query.Get("pole"))
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, parseErr.Error())
			return
		}
		apps, err = repo.TouchingPole(ctx, id)
	default:
		apps, err = repo.Live(ctx)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := ApplicationIndexJSON{Count: len(apps), Applications: make([]ApplicationSummaryJSON, 0, len(apps))}
	for _, app := range apps {
		out.Applications = append(out.Applications, summariseApplication(app))
	}
	writeJSON(w, http.StatusOK, out)
}

// ApplicationDetailJSON is one application with its history.
type ApplicationDetailJSON struct {
	ApplicationSummaryJSON
	PoleTags []string              `json:"pole_tags"`
	History  []ApplicationStepJSON `json:"history"`
}

// ApplicationStepJSON is one step of a history.
type ApplicationStepJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	On   string `json:"on"`
	By   string `json:"by"`
	Note string `json:"note,omitempty"`
}

func (s *Server) handleApplication(w http.ResponseWriter, r *http.Request) {
	id, err := PathParam(r.URL.Path, "/applications/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app, err := store.NewApplications(s.db).Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := ApplicationDetailJSON{ApplicationSummaryJSON: summariseApplication(app)}
	for _, p := range app.Poles {
		out.PoleTags = append(out.PoleTags, string(p))
	}
	for _, e := range app.History {
		out.History = append(out.History, ApplicationStepJSON{
			From: string(e.From), To: string(e.To),
			On: e.On.String(), By: e.By, Note: e.Note,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func summariseApplication(app *permit.Application) ApplicationSummaryJSON {
	return ApplicationSummaryJSON{
		ID: app.ID, Applicant: app.Applicant, Owner: app.Owner,
		State: string(app.State), HeldBy: string(app.HeldBy()),
		Live: app.Live(), Opened: app.Opened().String(), Poles: app.PoleCount(),
	}
}

// AllRoutes registers every endpoint this service carries, which is what the
// serve command wants and what a caller assembling a smaller service can avoid
// by registering the groups it wants instead.
func (s *Server) AllRoutes() *Server {
	return s.RegisterPlant().RegisterRates().RegisterApplications()
}

// HasRoute reports whether a path is registered, matched exactly.
func (s *Server) HasRoute(path string) bool {
	_, ok := s.routes[strings.TrimSpace(path)]
	return ok
}

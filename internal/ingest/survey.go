package ingest

import (
	"fmt"
	"io"
	"sort"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/survey"
	"github.com/glinskevxer/downguy/internal/unit"
)

// SurveyColumns are the columns a survey export has to carry. A survey file is
// one row per reading rather than one row per sheet, because that is what the
// field tablets produce, so the sheets have to be assembled from the rows.
var SurveyColumns = []string{"survey_id", "pole_id", "taken", "crew", "subject", "height", "method"}

// SurveyRow is one reading as the file carries it, before the sheets are
// assembled.
type SurveyRow struct {
	SurveyID  string
	Pole      plant.PoleID
	Taken     calendar.Date
	Crew      string
	Condition plant.Condition
	Ground    unit.Height
	Note      string
	Reading   survey.Measurement
}

// SurveyRows reads a survey export without assembling it, which is what an
// import that only wants to count or check the file needs.
func SurveyRows(r io.Reader) (Result[SurveyRow], error) {
	reader, err := NewReader(r)
	if err != nil {
		return Result[SurveyRow]{}, err
	}
	if missing := reader.Header().Require(SurveyColumns...); len(missing) > 0 {
		return Result[SurveyRow]{}, Missing("survey", missing)
	}
	return Each(reader, parseSurveyRow), nil
}

func parseSurveyRow(h *Header, row []string) (SurveyRow, error) {
	var out SurveyRow
	out.SurveyID = h.Field(row, "survey_id")
	if out.SurveyID == "" {
		return out, Bad("survey_id", "", fmt.Errorf("%w: a reading with no sheet", ErrBadFile))
	}

	pole, err := plant.ParsePoleID(h.Field(row, "pole_id"))
	if err != nil {
		return out, Bad("pole_id", h.Field(row, "pole_id"), err)
	}
	out.Pole = pole

	taken, err := calendar.ParseDate(h.Field(row, "taken"))
	if err != nil {
		return out, Bad("taken", h.Field(row, "taken"), err)
	}
	out.Taken = taken
	out.Crew = h.Field(row, "crew")

	out.Condition = plant.ConditionUnknown
	if text := h.Field(row, "condition"); text != "" {
		out.Condition = plant.Condition(text)
	}
	if text := h.Field(row, "ground_line"); text != "" {
		ground, err := unit.ParseHeight(text)
		if err != nil {
			return out, Bad("ground_line", text, err)
		}
		out.Ground = ground
	}
	out.Note = h.Field(row, "note")

	height, err := unit.ParseHeight(h.Field(row, "height"))
	if err != nil {
		return out, Bad("height", h.Field(row, "height"), err)
	}
	method, err := survey.ParseMethod(h.Field(row, "method"))
	if err != nil {
		return out, Bad("method", h.Field(row, "method"), err)
	}
	out.Reading = survey.Measurement{
		Subject: h.Field(row, "subject"),
		Height:  height,
		Method:  method,
		Crew:    out.Crew,
		Note:    h.Field(row, "reading_note"),
	}
	if err := out.Reading.Validate(); err != nil {
		return out, Bad("subject", out.Reading.Subject, err)
	}
	return out, nil
}

// Surveys reads a survey export and assembles it into sheets. Rows sharing a
// survey id become one sheet; a row that disagrees with the sheet's pole or
// date is rejected rather than quietly overwriting what the earlier rows said.
func Surveys(r io.Reader) (Result[survey.PoleSheet], error) {
	rows, err := SurveyRows(r)
	if err != nil {
		return Result[survey.PoleSheet]{}, err
	}
	out := Result[survey.PoleSheet]{Problems: rows.Problems}

	sheets := map[string]*survey.PoleSheet{}
	var order []string
	for _, row := range rows.Rows {
		sheet, seen := sheets[row.SurveyID]
		if !seen {
			sheet = &survey.PoleSheet{
				ID: row.SurveyID, Pole: row.Pole, Taken: row.Taken, Crew: row.Crew,
				Condition: row.Condition, GroundLine: row.Ground, Notes: row.Note,
			}
			sheets[row.SurveyID] = sheet
			order = append(order, row.SurveyID)
		} else if sheet.Pole != row.Pole || !sheet.Taken.Equal(row.Taken) {
			out.Problems = append(out.Problems, RowError{
				Column: "survey_id", Value: row.SurveyID,
				Err: fmt.Errorf("%w: %s already covers %s on %s", ErrBadFile,
					row.SurveyID, sheet.Pole, sheet.Taken),
			})
			continue
		}
		sheet.Measurements = append(sheet.Measurements, row.Reading)
	}

	sort.Strings(order)
	for _, id := range order {
		sheet := sheets[id]
		if err := sheet.Validate(); err != nil {
			out.Problems = append(out.Problems, RowError{
				Column: "survey_id", Value: id, Err: err,
			})
			continue
		}
		out.Rows = append(out.Rows, *sheet)
	}
	return out, nil
}

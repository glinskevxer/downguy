package ingest

import (
	"io"
	"strconv"

	"github.com/glinskevxer/downguy/internal/calendar"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/unit"
)

// PoleColumns are the columns a pole export has to carry.
var PoleColumns = []string{"pole_id", "length_ft", "class", "species", "owner"}

// Poles reads a pole export. Set depth, condition and the install year are
// optional: the export this utility receives from its mapping system does not
// carry them, and a standard set depth is a better answer than refusing the
// whole file.
func Poles(r io.Reader) (Result[plant.Pole], error) {
	reader, err := NewReader(r)
	if err != nil {
		return Result[plant.Pole]{}, err
	}
	if missing := reader.Header().Require(PoleColumns...); len(missing) > 0 {
		return Result[plant.Pole]{}, Missing("pole", missing)
	}
	return Each(reader, parsePole), nil
}

func parsePole(h *Header, row []string) (plant.Pole, error) {
	var pole plant.Pole

	id, err := plant.ParsePoleID(h.Field(row, "pole_id"))
	if err != nil {
		return pole, Bad("pole_id", h.Field(row, "pole_id"), err)
	}
	pole.ID = id

	length, err := strconv.Atoi(h.Field(row, "length_ft"))
	if err != nil {
		return pole, Bad("length_ft", h.Field(row, "length_ft"), err)
	}
	pole.Length = length
	pole.Class = plant.Class(h.Field(row, "class"))

	species, err := plant.ParseSpecies(h.Field(row, "species"))
	if err != nil {
		return pole, Bad("species", h.Field(row, "species"), err)
	}
	pole.Species = species
	pole.Owner = h.Field(row, "owner")

	pole.SetDepth = plant.StandardSetDepth(length)
	if text := h.Field(row, "set_depth"); text != "" {
		depth, err := unit.ParseHeight(text)
		if err != nil {
			return pole, Bad("set_depth", text, err)
		}
		pole.SetDepth = depth
	}

	pole.Condition = plant.ConditionUnknown
	if text := h.Field(row, "condition"); text != "" {
		pole.Condition = plant.Condition(text)
	}

	if text := h.Field(row, "installed_in"); text != "" {
		year, err := strconv.Atoi(text)
		if err != nil {
			return pole, Bad("installed_in", text, err)
		}
		pole.InstalledIn = year
	}
	if text := h.Field(row, "latitude"); text != "" {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return pole, Bad("latitude", text, err)
		}
		pole.Latitude = value
	}
	if text := h.Field(row, "longitude"); text != "" {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return pole, Bad("longitude", text, err)
		}
		pole.Longitude = value
	}

	if err := pole.Validate(); err != nil {
		return pole, Bad("pole_id", string(pole.ID), err)
	}
	return pole, nil
}

// AttachmentColumns are the columns an attachment export has to carry.
var AttachmentColumns = []string{"attachment_id", "pole_id", "owner", "kind", "height"}

// Attachments reads an attachment export.
func Attachments(r io.Reader) (Result[plant.Attachment], error) {
	reader, err := NewReader(r)
	if err != nil {
		return Result[plant.Attachment]{}, err
	}
	if missing := reader.Header().Require(AttachmentColumns...); len(missing) > 0 {
		return Result[plant.Attachment]{}, Missing("attachment", missing)
	}
	return Each(reader, parseAttachment), nil
}

func parseAttachment(h *Header, row []string) (plant.Attachment, error) {
	var att plant.Attachment
	att.ID = h.Field(row, "attachment_id")

	pole, err := plant.ParsePoleID(h.Field(row, "pole_id"))
	if err != nil {
		return att, Bad("pole_id", h.Field(row, "pole_id"), err)
	}
	att.Pole = pole
	att.Owner = h.Field(row, "owner")

	kind, err := plant.ParseKind(h.Field(row, "kind"))
	if err != nil {
		return att, Bad("kind", h.Field(row, "kind"), err)
	}
	att.Kind = kind

	height, err := unit.ParseHeight(h.Field(row, "height"))
	if err != nil {
		return att, Bad("height", h.Field(row, "height"), err)
	}
	att.Height = height

	att.Authorized = truthy(h.Field(row, "authorized"))

	if text := h.Field(row, "in_force_from"); text != "" {
		from, err := calendar.ParseDate(text)
		if err != nil {
			return att, Bad("in_force_from", text, err)
		}
		att.InForce.From = from
	}
	if text := h.Field(row, "in_force_to"); text != "" {
		to, err := calendar.ParseDate(text)
		if err != nil {
			return att, Bad("in_force_to", text, err)
		}
		att.InForce.To = to
	}
	if text := h.Field(row, "voltage"); text != "" {
		volts, err := strconv.Atoi(text)
		if err != nil {
			return att, Bad("voltage", text, err)
		}
		att.Voltage = volts
	}
	if text := h.Field(row, "face"); text != "" {
		face, err := plant.ParseFace(text)
		if err != nil {
			return att, Bad("face", text, err)
		}
		att.Face = face
	}
	att.Note = h.Field(row, "note")

	if err := att.Validate(); err != nil {
		return att, Bad("attachment_id", att.ID, err)
	}
	return att, nil
}

// SpanColumns are the columns a span export has to carry.
var SpanColumns = []string{"span_id", "from_pole", "to_pole", "length_ft", "cover"}

// Spans reads a span export.
func Spans(r io.Reader) (Result[plant.SpanRecord], error) {
	reader, err := NewReader(r)
	if err != nil {
		return Result[plant.SpanRecord]{}, err
	}
	if missing := reader.Header().Require(SpanColumns...); len(missing) > 0 {
		return Result[plant.SpanRecord]{}, Missing("span", missing)
	}
	return Each(reader, parseSpan), nil
}

func parseSpan(h *Header, row []string) (plant.SpanRecord, error) {
	var span plant.SpanRecord
	span.ID = h.Field(row, "span_id")

	from, err := plant.ParsePoleID(h.Field(row, "from_pole"))
	if err != nil {
		return span, Bad("from_pole", h.Field(row, "from_pole"), err)
	}
	span.FromPole = from

	to, err := plant.ParsePoleID(h.Field(row, "to_pole"))
	if err != nil {
		return span, Bad("to_pole", h.Field(row, "to_pole"), err)
	}
	span.ToPole = to

	length, err := unit.ParseSpan(h.Field(row, "length_ft"))
	if err != nil {
		return span, Bad("length_ft", h.Field(row, "length_ft"), err)
	}
	span.Length = length

	cover, err := plant.ParseCover(h.Field(row, "cover"))
	if err != nil {
		return span, Bad("cover", h.Field(row, "cover"), err)
	}
	span.Cover = cover

	if text := h.Field(row, "drop"); text != "" {
		drop, err := unit.ParseHeight(text)
		if err != nil {
			return span, Bad("drop", text, err)
		}
		span.Drop = drop
	}
	span.Dead = truthy(h.Field(row, "dead_end"))

	if err := span.Validate(); err != nil {
		return span, Bad("span_id", span.ID, err)
	}
	return span, nil
}

// truthy reads the several ways the systems feeding this one write a yes.
func truthy(text string) bool {
	switch normalise(text) {
	case "y", "yes", "t", "true", "1":
		return true
	}
	return false
}

func normalise(text string) string { return FoldColumn(text) }

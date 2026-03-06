package cliapp

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/plant"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "export",
		Summary: "write plant back out as csv",
		Usage:   exportUsage(),
		Run:     runExport,
	})
}

// exportKinds are what this command can write.
var exportKinds = []string{"attachments"}

func exportUsage() string {
	f := newExportFlags(nil)
	return f.Usage("export <kind> [options]", "export <kind> [options]",
		"Writes plant back out in the same shape the importer reads, so an export\n"+
			"can be corrected in a spreadsheet and fed straight back in.\n\n"+
			"The kind is one of "+strings.Join(exportKinds, ", ")+".")
}

func newExportFlags(out io.Writer) *Flags {
	f := NewFlags("export", out)
	f.Set().String("pole", "", "only what is on one pole")
	f.Set().String("owner", "", "only what belongs to one attacher")
	f.Set().Bool("unauthorized", false, "only what nobody has permission for")
	return f
}

func runExport(env *Env) int {
	f := newExportFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) != 1 {
		return env.Fail(CodeUsage, "export needs a kind: %s", strings.Join(exportKinds, ", "))
	}
	kind := strings.ToLower(rest[0])
	if kind != "attachments" {
		return env.Fail(CodeUsage, "no exporter for %q, try one of %s", kind, strings.Join(exportKinds, ", "))
	}

	poleTag := f.Set().Lookup("pole").Value.String()
	owner := f.Set().Lookup("owner").Value.String()
	onlyUnauthorized := flagBool(f, "unauthorized")
	chosen := 0
	for _, on := range []bool{poleTag != "", owner != "", onlyUnauthorized} {
		if on {
			chosen++
		}
	}
	if chosen > 1 {
		return env.Fail(CodeUsage, "export takes at most one of -pole, -owner and -unauthorized")
	}

	ctx := context.Background()
	db, _, err := f.Open(ctx, env)
	if err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	defer func() { _ = db.Close() }()

	loader := store.NewPlant(db)
	var atts []plant.Attachment
	switch {
	case poleTag != "":
		id, err := plant.ParsePoleID(poleTag)
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		atts, err = loader.Attachments().OnPole(ctx, id)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	case owner != "":
		atts, err = loader.Attachments().OwnedBy(ctx, owner)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	case onlyUnauthorized:
		atts, err = loader.Attachments().Unauthorized(ctx)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	default:
		atts, err = everyAttachment(ctx, loader)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
	}

	if err := ingest.WriteAttachments(env.Stdout, atts); err != nil {
		return env.Fail(CodeInternal, "%v", err)
	}
	if len(atts) == 0 {
		return CodeFindings
	}
	return CodeOK
}

// everyAttachment walks the poles and collects what is on them, in pole then
// height order, so an export of the whole system reads the same twice.
func everyAttachment(ctx context.Context, loader *store.Plant) ([]plant.Attachment, error) {
	poles, err := loader.Poles().All(ctx)
	if err != nil {
		return nil, err
	}
	var out []plant.Attachment
	for _, pole := range poles {
		atts, err := loader.Attachments().OnPole(ctx, pole.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, atts...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pole != out[j].Pole {
			return out[i].Pole < out[j].Pole
		}
		if out[i].Height != out[j].Height {
			return out[i].Height > out[j].Height
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

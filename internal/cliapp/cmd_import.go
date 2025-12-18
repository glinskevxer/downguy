package cliapp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/glinskevxer/downguy/internal/ingest"
	"github.com/glinskevxer/downguy/internal/store"
)

func init() {
	Register(Command{
		Name:    "import",
		Summary: "load a plant or survey export into the database",
		Usage:   importUsage(),
		Run:     runImport,
	})
}

// importKinds are the files this command knows how to read.
var importKinds = []string{"poles", "attachments", "spans", "surveys"}

func importUsage() string {
	f := newImportFlags(nil)
	return f.Usage("import <kind> [file]",
		"import <kind> [file]",
		"Reads a csv export and files it. The kind is one of "+strings.Join(importKinds, ", ")+".\n"+
			"With no file it reads standard input.\n\n"+
			"Rows that will not parse are reported with their row number and the rest of\n"+
			"the file is still loaded, because a fifteen thousand row plant export always\n"+
			"has a handful of bad rows and refusing the file helps nobody.")
}

func newImportFlags(out io.Writer) *Flags {
	f := NewFlags("import", out)
	f.Set().Bool("dry-run", false, "read and check the file without writing anything")
	f.Set().Int("max-problems", 20, "how many bad rows to print before summarising")
	return f
}

func runImport(env *Env) int {
	f := newImportFlags(env.Stderr)
	if err := f.Parse(env.Args); err != nil {
		return CodeUsage
	}
	rest := f.Rest()
	if len(rest) == 0 {
		return env.Fail(CodeUsage, "import needs a kind: %s", strings.Join(importKinds, ", "))
	}
	if len(rest) > 2 {
		return env.Fail(CodeUsage, "import takes a kind and at most one file")
	}
	kind := strings.ToLower(rest[0])
	if !knownImportKind(kind) {
		return env.Fail(CodeUsage, "no importer for %q, try one of %s", kind, strings.Join(importKinds, ", "))
	}

	source := env.Stdin
	if len(rest) == 2 {
		file, err := os.Open(rest[1])
		if err != nil {
			return env.Fail(CodeUsage, "%v", err)
		}
		defer func() { _ = file.Close() }()
		source = file
	}

	dryRun := flagBool(f, "dry-run")
	maxProblems := flagInt(f, "max-problems", 20)

	ctx := context.Background()
	var db *store.Plant
	if !dryRun {
		handle, _, err := f.Open(ctx, env)
		if err != nil {
			return env.Fail(CodeInternal, "%v", err)
		}
		defer func() { _ = handle.Close() }()
		db = store.NewPlant(handle)
	}

	written, problems, err := importOne(ctx, kind, source, db)
	if err != nil {
		return env.Fail(CodeUsage, "%v", err)
	}

	verb := "filed"
	if dryRun {
		verb = "would file"
	}
	env.Print("%s %d %s\n", verb, written, kind)
	if len(problems) == 0 {
		return CodeOK
	}
	shown := problems
	if len(shown) > maxProblems {
		shown = shown[:maxProblems]
	}
	for _, p := range shown {
		fmt.Fprintf(env.Stderr, "  %s\n", p)
	}
	if len(problems) > len(shown) {
		fmt.Fprintf(env.Stderr, "  and %d more\n", len(problems)-len(shown))
	}
	env.Print("%d rows rejected\n", len(problems))
	return CodeFindings
}

func knownImportKind(kind string) bool {
	for _, k := range importKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// importOne reads one file and files what it produced. It returns how many
// records were written rather than how many parsed, because a record that
// parsed and then failed a foreign key is not a record anybody has.
func importOne(ctx context.Context, kind string, source io.Reader, db *store.Plant) (int, []ingest.RowError, error) {
	switch kind {
	case "poles":
		result, err := ingest.Poles(source)
		if err != nil {
			return 0, nil, err
		}
		if db == nil {
			return result.Count(), result.Problems, nil
		}
		written := 0
		problems := result.Problems
		for _, pole := range result.Rows {
			if err := db.Poles().Put(ctx, pole); err != nil {
				problems = append(problems, ingest.RowError{Column: "pole_id", Value: string(pole.ID), Err: err})
				continue
			}
			written++
		}
		return written, problems, nil

	case "attachments":
		result, err := ingest.Attachments(source)
		if err != nil {
			return 0, nil, err
		}
		if db == nil {
			return result.Count(), result.Problems, nil
		}
		written := 0
		problems := result.Problems
		for _, att := range result.Rows {
			if err := db.Attachments().Put(ctx, att); err != nil {
				problems = append(problems, ingest.RowError{Column: "attachment_id", Value: att.ID, Err: err})
				continue
			}
			written++
		}
		return written, problems, nil

	case "spans":
		result, err := ingest.Spans(source)
		if err != nil {
			return 0, nil, err
		}
		if db == nil {
			return result.Count(), result.Problems, nil
		}
		written := 0
		problems := result.Problems
		for _, span := range result.Rows {
			if err := db.Spans().Put(ctx, span); err != nil {
				problems = append(problems, ingest.RowError{Column: "span_id", Value: span.ID, Err: err})
				continue
			}
			written++
		}
		return written, problems, nil

	case "surveys":
		result, err := ingest.Surveys(source)
		if err != nil {
			return 0, nil, err
		}
		if db == nil {
			return result.Count(), result.Problems, nil
		}
		surveys := store.NewSurveys(db.Poles().DB())
		written := 0
		problems := result.Problems
		for _, sheet := range result.Rows {
			if err := surveys.Put(ctx, sheet); err != nil {
				problems = append(problems, ingest.RowError{Column: "survey_id", Value: sheet.ID, Err: err})
				continue
			}
			written++
		}
		return written, problems, nil
	}
	return 0, nil, fmt.Errorf("no importer for %q", kind)
}

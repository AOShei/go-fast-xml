package gofastxml

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml/internal/draft"
	"github.com/AOShei/go-fast-xml/internal/xsd"
)

// Draft is a draft profile made from an XML Schema by DraftProfile, for a
// person to finish. It is not a profile: each rule carries
// "review": true, which LoadProfile refuses.
type Draft struct {
	// JSON is the draft, in the profile format (PROFILE_FORMAT.md).
	JSON []byte
	// Files are the schema documents read, the named one first.
	Files []string
	// Warnings say what of the schema was not read: a location that is a
	// URL or names no file on disk (input.external_reference), a limit
	// that left documents out (limit.exceeded). The draft's notes say the
	// same.
	Warnings []gofastcore.Warning
}

// DraftProfile reads the XML Schema (XSD) at path, with the documents it
// includes, imports and redefines that are on disk relative to it, and
// writes a draft profile from what the schema declares. Nothing is
// fetched. DTDs and RELAX NG schemas are not read. The error is for the
// named file only: one that cannot be read or is not a schema.
func DraftProfile(ctx context.Context, path string) (*Draft, error) {
	set, err := xsd.Load(ctx, path, xsd.DefaultLimits)
	if err != nil {
		return nil, fmt.Errorf("gofastxml: %w", err)
	}
	model := set.Analyze()
	d := &Draft{}
	for _, doc := range set.Docs {
		d.Files = append(d.Files, doc.Path)
	}
	var problems []string
	external := 0
	for _, p := range set.Problems {
		var msg string
		if p.Kind == xsd.LimitCut {
			msg = p.Msg
			if p.Location != "" {
				msg = p.Location + ": " + msg
			}
			d.Warnings = append(d.Warnings, gofastcore.Warning{Code: gofastcore.WarnLimitExceeded, Severity: gofastcore.SeverityError, Message: "schema: " + msg})
		} else {
			from, err := filepath.Rel(filepath.Dir(path), p.From)
			if err != nil {
				from = p.From
			}
			msg = fmt.Sprintf("%s %q in %s: %s", p.What, p.Location, filepath.ToSlash(from), p.Msg)
			if p.Kind != xsd.Unreadable {
				external++
			}
		}
		problems = append(problems, msg)
	}
	if external > 0 {
		d.Warnings = append(d.Warnings, gofastcore.Warning{Code: gofastcore.WarnInputExternalReference, Severity: gofastcore.SeverityWarning,
			Message: fmt.Sprintf("%s not read: a URL, an absolute location, no location or no such file; nothing is fetched (the draft's notes list them)", plural(external, "schema location", "schema locations"))})
	}
	if model.Cut {
		d.Warnings = append(d.Warnings, gofastcore.Warning{Code: gofastcore.WarnLimitExceeded, Severity: gofastcore.SeverityError,
			Message: fmt.Sprintf("schema: more than %d element declarations; the rest are not in the draft", model.MaxDecls)})
	}
	d.JSON = draft.Write(draft.Input{Entry: path, Set: set, Model: model, Problems: problems})
	return d, nil
}

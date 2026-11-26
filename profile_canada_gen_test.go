package gofastxml

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// profiles/canada-laws.json is generated from the tables below, so that
// the lims: attributes every provision carries are written the same way
// on each rule. go test -run TestCanadaGenerated -canada.update rewrites
// it; without the flag the test fails when the file differs.
var canadaUpdate = flag.Bool("canada.update", false, "rewrite profiles/canada-laws.json")

const limsNS = "http://justice.gc.ca/lims"

type jrule = map[string]any

func attr(name string) map[string]string { return map[string]string{"attr": name} }

// canadaLims are the lims: attributes kept as written in a block's Attrs.
// lims:id is the anchor; lims:pit-date and lims:current-date are on the
// root only (its Extra) but kept here too in case a provision has them.
var canadaLims = []string{"lims:inforce-start-date", "lims:inforce-end-date", "lims:lastAmendedDate",
	"lims:enacted-date", "lims:enactId", "lims:fid", "lims:pit-date", "lims:current-date"}

// canadaPlain are the unprefixed attributes kept, under canada.<name>:
// what they declare (in-force state, kind, status), never evaluated.
var canadaPlain = []string{"type", "in-force", "status", "target", "style", "bilingual", "spanlanguages", "placement",
	"salutation", "generate-in-text", "orientation", "list-item", "id"}

func canadaAttrs(element string) map[string]any {
	m := map[string]any{}
	if element != "" {
		m["canada.element"] = element
	}
	for _, a := range canadaLims {
		m[a] = attr(a)
	}
	for _, a := range canadaPlain {
		m["canada."+a] = attr(a)
	}
	return m
}

// prov is a provision or container: no block of its own, anchor lims:id.
func prov(match, as, element string) jrule {
	return jrule{"match": match, "as": as, "anchor": attr("lims:id"), "attrs": canadaAttrs(element)}
}

func para(match, element string) jrule {
	return jrule{"match": match, "as": "block", "kind": "paragraph", "anchor": attr("lims:id"), "attrs": canadaAttrs(element)}
}

func meta(match, key string, extra map[string]any) jrule {
	r := jrule{"match": match, "as": "metadata", "field": "extra", "key": key}
	if extra != nil {
		r["extra"] = extra
	}
	return r
}

func canadaRules() []jrule {
	rootExtra := map[string]any{}
	for _, a := range canadaLims {
		rootExtra["lims."+a[len("lims:"):]] = attr(a)
	}
	rootExtra["lims.id"] = attr("lims:id")
	for _, a := range []string{"bill-origin", "bill-type", "in-force", "hasPreviousVersion", "gazette-part", "regulation-type",
		"startdate", "endtdate", "reverseTitle", "alpha"} {
		rootExtra["lims."+a] = attr(a)
	}
	var rs []jrule
	for _, root := range []string{"Statute", "Regulation", "Bill"} {
		// A metadata rule whose value is an attribute reads its children
		// by their own rules: the root is transparent and gives the
		// language.
		rs = append(rs, jrule{"match": "/" + root, "as": "metadata", "field": "language", "value": attr("xml:lang"),
			"extra": rootExtra, "note": "transparent; xml:lang is Metadata.Language"})
	}
	// Identification.
	rs = append(rs,
		jrule{"match": "Identification", "as": "transparent"},
		jrule{"match": "Identification/LongTitle", "as": "metadata", "field": "title", "also_heading": 1},
		meta("Identification/ShortTitle", "lims.ShortTitle", map[string]any{"lims.ShortTitle.status": attr("status")}),
		meta("Identification/RunningHead", "lims.RunningHead", nil),
		meta("Identification/BillNumber", "lims.BillNumber", nil),
		jrule{"match": "Identification/Parliament", "as": "transparent"},
		meta("Parliament/Session", "lims.Parliament.Session", nil),
		meta("Parliament/Number", "lims.Parliament.Number", nil),
		meta("Parliament/Year-s", "lims.Parliament.Year-s", nil),
		jrule{"match": "Parliament/RegnalYear", "as": "transparent"},
		meta("RegnalYear/Year-s", "lims.RegnalYear.Year-s", nil),
		meta("RegnalYear/Monarch", "lims.RegnalYear.Monarch", nil),
		jrule{"match": "Identification/BillHistory", "as": "transparent"},
		meta("BillHistory/Stages", "lims.Stages", map[string]any{"lims.Stages.stage": attr("stage")}),
		jrule{"match": "Identification/Chapter", "as": "transparent"},
		meta("Chapter/ConsolidatedNumber", "lims.ConsolidatedNumber", map[string]any{"lims.ConsolidatedNumber.official": attr("official")}),
		jrule{"match": "Chapter/AnnualStatuteId", "as": "transparent", "extra": map[string]any{"lims.AnnualStatuteId.revised-statute": attr("revised-statute")}},
		meta("AnnualStatuteId/AnnualStatuteNumber", "lims.AnnualStatuteNumber", nil),
		meta("AnnualStatuteId/YYYY", "lims.AnnualStatuteId.YYYY", nil),
		meta("Identification/BillRefNumber", "lims.BillRefNumber", map[string]any{"lims.BillRefNumber.date-time": attr("date-time")}),
		meta("Identification/InstrumentNumber", "lims.InstrumentNumber", nil),
		meta("Identification/RegistrationDate", "lims.RegistrationDate", nil),
		meta("Identification/ConsolidationDate", "lims.ConsolidationDate", nil),
		meta("Identification/LastAmendedDate", "lims.LastAmendedDate", nil),
		meta("Identification/LastModifiedDate", "lims.LastModifiedDate", nil),
		meta("Identification/LimsAuthority", "lims.LimsAuthority", nil),
		meta("Identification/StatuteYear", "lims.StatuteYear", nil),
		jrule{"match": "Identification/RegulationMakerOrder", "as": "transparent"},
		meta("RegulationMakerOrder/RegulationMaker", "lims.RegulationMaker", nil),
		meta("RegulationMakerOrder/OrderNumber", "lims.OrderNumber", nil),
		meta("RegulationMakerOrder/Date", "lims.RegulationMakerOrder.Date", nil),
		jrule{"match": "Identification/EnablingAuthority", "as": "block", "kind": "paragraph", "anchor": attr("lims:id"),
			"attrs": canadaAttrs("EnablingAuthority"), "extra": map[string]any{"lims.EnablingAuthority.link": map[string]string{"attr": "link", "child": "XRefExternal"}}},
		prov("ReaderNote", "transparent", "ReaderNote"),
		jrule{"match": "Date", "as": "inline", "space": true},
		jrule{"match": "Date/YYYY", "as": "inline", "space": true},
		jrule{"match": "Date/MM", "as": "inline", "space": true},
		jrule{"match": "Date/DD", "as": "inline", "space": true},
	)
	// Containers and provisions: no block of their own.
	for _, e := range []string{"Introduction", "Enacts", "Preamble", "Order", "Body", "BillPiece", "RegulationPiece", "SectionPiece",
		"Section", "Subsection", "Paragraph", "Subparagraph", "Clause", "Subclause", "Subsubclause", "Definition", "Provision",
		"ContinuedSectionSubsection", "ContinuedParagraph", "ContinuedSubparagraph", "ContinuedClause", "ContinuedSubclause",
		"ContinuedDefinition", "ContinuedFormulaParagraph", "FormulaGroup", "Formula", "FormulaDefinition", "FormulaParagraph",
		"Oath", "QuotedText", "FormGroup", "ImageGroup", "TableGroup", "BilingualGroup", "SignatureBlock"} {
		rs = append(rs, prov(e, "transparent", e))
	}
	for _, e := range []string{"Summary", "Recommendation", "Schedule", "DocumentInternal", "ConventionAgreementTreaty", "Group"} {
		rs = append(rs, prov(e, "section", e))
	}
	// Text of amendments and related provisions: every block inside is marked.
	for _, e := range []string{"AmendedText", "ReadAsText", "AmendedContent"} {
		r := prov(e, "transparent", e)
		r["mark"] = map[string]any{"canada.amendment": e, "canada.amendment_lang": attr("xml:lang")}
		rs = append(rs, r)
	}
	for _, e := range []string{"RelatedOrNotInForce", "RelatedProvision"} {
		r := prov(e, "transparent", e)
		r["mark"] = map[string]any{"canada.related": e}
		rs = append(rs, r)
	}
	heading := func(match string, level any, element string) jrule {
		return jrule{"match": match, "as": "heading", "level": level, "anchor": attr("lims:id"), "attrs": canadaAttrs(element)}
	}
	outOfLine := func(match, element string) jrule {
		r := para(match, element)
		r["out_of_line"] = true
		return r
	}
	rs = append(rs,
		heading("Heading", attr("level"), "Heading"),
		heading("GroupHeading", "nesting", "GroupHeading"),
		heading("ScheduleFormHeading", "nesting", "ScheduleFormHeading"),
		heading("Summary/TitleText", "nesting", "TitleText"),
		heading("Recommendation/TitleText", "nesting", "TitleText"),
		jrule{"match": "TitleText", "as": "inline"},
		jrule{"match": "ScheduleFormHeading/TitleText", "as": "inline", "space": true},
		jrule{"match": "BilingualGroup/TitleText", "as": "block", "kind": "caption", "anchor": attr("lims:id"), "attrs": canadaAttrs("TitleText")},
		// No canada.element of its own: the heading is the first block
		// of its provision and carries the provision's element name.
		heading("MarginalNote", 6, ""),
		outOfLine("Heading/MarginalNote", "MarginalNote"),
		outOfLine("Heading/Note", "Note"),
		outOfLine("Heading/HistoricalNote", "HistoricalNote"),
		outOfLine("ScheduleFormHeading/OriginatingRef", "OriginatingRef"),
		outOfLine("ScheduleFormHeading/HistoricalNote", "HistoricalNote"),
		para("OriginatingRef", "OriginatingRef"),
		outOfLine("HistoricalNote", "HistoricalNote"),
		jrule{"match": "HistoricalNoteSubItem", "as": "block", "kind": "paragraph", "note": "a line of its HistoricalNote"},
		jrule{"match": "Label", "as": "label"},
		jrule{"match": "FormulaTerm", "as": "label"},
		para("Text", ""),
		jrule{"match": "FormulaText", "as": "block", "kind": "formula", "notation": "plain", "anchor": attr("lims:id"), "attrs": canadaAttrs("FormulaText")},
		para("FormulaConnector", "FormulaConnector"),
		para("Repealed", "Repealed"),
		jrule{"match": "Text/Repealed", "as": "inline"},
		jrule{"match": "TitleText/Repealed", "as": "inline"},
		jrule{"match": "MarginalNote/Repealed", "as": "inline"},
		jrule{"match": "Label/Repealed", "as": "inline"},
		para("Reserved", "Reserved"),
		jrule{"match": "Text/Reserved", "as": "inline"},
		para("Note", "Note"),
		para("LongTitle", "LongTitle"),
		para("ShortTitle", "ShortTitle"),
		para("RunningHead", "RunningHead"),
		para("AlternateText", "AlternateText"),
		para("BilingualItemEn", "BilingualItemEn"),
		para("BilingualItemFr", "BilingualItemFr"),
		para("SignatureLine", "SignatureLine"),
		para("SignatureTitle", "SignatureTitle"),
		para("SignatureName", "SignatureName"),
		jrule{"match": "Footnote", "as": "block", "kind": "footnote", "anchor": attr("id"), "attrs": canadaAttrs("")},
		jrule{"match": "Caption", "as": "block", "kind": "caption", "anchor": attr("lims:id"), "attrs": canadaAttrs("")},
		jrule{"match": "table/title", "as": "block", "kind": "caption", "anchor": attr("lims:id"), "attrs": canadaAttrs("")},
		jrule{"match": "table", "as": "table", "model": "cals", "anchor": attr("lims:id"), "attrs": canadaAttrs("")},
		jrule{"match": "List", "as": "list", "ordered": false, "anchor": attr("lims:id"), "attrs": canadaAttrs("")},
		jrule{"match": "Item", "as": "item", "anchor": attr("lims:id"), "attrs": canadaAttrs("")},
		jrule{"match": "Image", "as": "image", "id": attr("source")},
		jrule{"match": "MathMLBlock", "as": "block", "kind": "formula", "notation": "plain", "attrs": map[string]any{"canada.math": "mathml"}},
		jrule{"match": "MathML", "as": "inline"},
		jrule{"match": "math", "as": "inline"},
		jrule{"match": "mml:math", "as": "inline"},
		jrule{"match": "RecentAmendments", "as": "table", "attrs": map[string]any{"canada.element": "RecentAmendments"}},
		jrule{"match": "RecentAmendments/Amendment", "as": "row"},
		jrule{"match": "Amendment/AmendmentCitation", "as": "cell"},
		jrule{"match": "Amendment/AmendmentDate", "as": "cell"},
		jrule{"match": "LineBreak", "as": "break"},
		jrule{"match": "PageBreak", "as": "transparent", "note": "empty; a page break is layout"},
		// Inline.
		jrule{"match": "Emphasis", "as": "inline", "attrs": map[string]any{"canada.style": attr("style")}},
		jrule{"match": "Emphasis[@style='italic']", "as": "inline", "style": "italic"},
		jrule{"match": "Emphasis[@style='bold']", "as": "inline", "style": "bold"},
		jrule{"match": "Sup", "as": "inline", "style": "superscript"},
		jrule{"match": "Sub", "as": "inline", "style": "subscript"},
		jrule{"match": "XRefExternal", "as": "inline", "link": attr("link"),
			"attrs": map[string]any{"canada.reference-type": attr("reference-type"), "canada.reference-level": attr("reference-level"), "canada.isURL": attr("isURL")}},
		jrule{"match": "XRefInternal", "as": "inline"},
		jrule{"match": "XRefSection", "as": "inline", "attrs": map[string]any{"canada.lawid": attr("lawid")}},
		jrule{"match": "FootnoteRef", "as": "inline", "link": attr("idref")},
		jrule{"match": "DefinedTermEn", "as": "inline"},
		jrule{"match": "DefinedTermFr", "as": "inline"},
		jrule{"match": "DefinitionRef", "as": "inline"},
		jrule{"match": "DefinitionEnOnly", "as": "inline"},
		jrule{"match": "DefinitionFrOnly", "as": "inline"},
		jrule{"match": "Language", "as": "inline", "attrs": map[string]any{"canada.lang": attr("xml:lang")}},
		jrule{"match": "Leader", "as": "inline", "space": true},
		jrule{"match": "LeaderRightJustified", "as": "inline", "space": true},
		jrule{"match": "FormBlank", "as": "inline"},
		jrule{"match": "InlineFont", "as": "inline"},
		jrule{"match": "Keep", "as": "inline"},
		jrule{"match": "Fraction", "as": "inline", "space": true},
		jrule{"match": "Numerator", "as": "inline", "space": true},
		jrule{"match": "Denominator", "as": "inline", "space": true},
	)
	return rs
}

func canadaProfileJSON() []byte {
	head := map[string]any{
		"profile":    "canada-laws",
		"title":      "Justice Canada consolidated federal acts, regulations and annual statutes (laws-lois-xml)",
		"format":     1,
		"namespaces": map[string]string{"lims": limsNS, "mml": "http://www.w3.org/1998/Math/MathML"},
		"match": []map[string]string{
			{"root": "Statute", "declares": limsNS},
			{"root": "Regulation", "declares": limsNS},
			{"root": "Bill", "declares": limsNS},
			{"root": "Statute", "doctype_public": "-//Justice Canada//"},
			{"root": "Regulation", "doctype_public": "-//Justice Canada//"},
			{"root": "Bill", "doctype_public": "-//Justice Canada//"},
		},
		"heading_base": 2,
	}
	var b bytes.Buffer
	b.WriteString("{\n")
	for _, k := range []string{"profile", "title", "format", "namespaces", "match", "heading_base"} {
		v, _ := json.Marshal(head[k])
		b.WriteString("  \"" + k + "\": " + string(v) + ",\n")
	}
	b.WriteString("  \"rules\": [\n")
	rs := canadaRules()
	for i, r := range rs {
		v, _ := json.Marshal(r)
		b.WriteString("    " + string(v))
		if i < len(rs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("  ]\n}\n")
	return b.Bytes()
}

func TestCanadaGenerated(t *testing.T) {
	path := filepath.Join("profiles", "canada-laws.json")
	want := canadaProfileJSON()
	if *canadaUpdate {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the generator: go test -run TestCanadaGenerated -canada.update", path)
	}
}

package gofastxml

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// The corpus, oracle and fixture helpers are in profile_patent_test.go.

func TestCDAFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", "cda", "note.xml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractBytes(context.Background(), data, &gofastcore.Options{Spans: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	patentExtra(t, doc, map[string]string{
		ExtraProfile: "cda", "cda.realm": "US", "cda.type_id_extension": "POCD_HD000040",
		"cda.template_id": "2.16.840.1.113883.10.20.22.1.1", "cda.id_root": "2.16.840.1.113883.19.5", "cda.id_extension": "DOC-1",
		"cda.code": "11488-4", "cda.code_display": "Consultation note", "cda.effective_time": "20260131120000-0500",
		"cda.confidentiality": "N", "cda.record_target": "Ada Example", "cda.author": "Ben Doctor",
		"cda.patient_id_root": "2.16.840.1.113883.19.5", "cda.patient_id_extension": "P-1", "cda.patient_gender": "F",
		"cda.patient_birth_time": "19800101", "cda.author_time": "20260131",
	})
	if doc.Metadata.Title != "Consultation Note" || doc.Metadata.Language != "en-US" {
		t.Errorf("metadata %+v", doc.Metadata)
	}
	patentCheck(t, doc, `heading1:Consultation Note
note~:2.16.840.1.113883.5.1
note~:2.16.840.1.113883.4.6
note~:999
note:2.16.840.1.113883.10.20.22.2.5.1
note:Problem list
heading2:Problems
paragraph:Known asthma, see table1.
caption:Active
list_item1(1.):Asthma
list_item1(2.):Hay fever
caption:Labs
table:
footnote:Reported by the patient.
image:Chest film
note~:OBS
note~:EVN
note~:64572001
note~:#prob1
heading3:History
paragraph~:No surgery.
`, `heading #
note # attribute=codeSystem element=ClinicalDocument/recordTarget/patientRole/patient/administrativeGenderCode type=attribute
note # attribute=root element=ClinicalDocument/author/assignedAuthor/id type=attribute
note # attribute=extension element=ClinicalDocument/author/assignedAuthor/id type=attribute
note # anchor_parent=sec-1 attribute=root cda.field=templateId type=attribute
note # anchor_parent=sec-1 attribute=displayName cda.code=11450-4 cda.code_system=2.16.840.1.113883.6.1 cda.field=code type=attribute
heading # anchor_parent=sec-1
paragraph #par-1 cda.part=narrative
caption # anchor_parent=lst-1 cda.part=narrative
list_item #it-1 cda.part=narrative
list_item # anchor_parent=lst-1 cda.part=narrative
caption # anchor_parent=tab-1 cda.part=narrative
table #tab-1 cda.border=1 cda.part=narrative
footnote #fn-1 cda.part=narrative
image # anchor_parent=sec-1 cda.part=narrative
note # anchor_parent=sec-1 attribute=classCode cda.entry_type=DRIV cda.part=entry element=ClinicalDocument/component/structuredBody/component/section/entry/observation type=attribute
note # anchor_parent=sec-1 attribute=moodCode cda.entry_type=DRIV cda.part=entry element=ClinicalDocument/component/structuredBody/component/section/entry/observation type=attribute
note # anchor_parent=sec-1 attribute=code cda.entry_type=DRIV cda.part=entry element=ClinicalDocument/component/structuredBody/component/section/entry/observation/code type=attribute
note # anchor_parent=sec-1 attribute=value cda.entry_type=DRIV cda.part=entry element=ClinicalDocument/component/structuredBody/component/section/entry/observation/text/reference type=attribute
heading # anchor_parent=sec-1
paragraph # anchor_parent=sec-1 cda.part=narrative element=ClinicalDocument/component/structuredBody/component/section/component/section/text
`)
	bs := blocks(doc)
	if tb := bs[12].Table; tb == nil || tb.HeaderRows != 1 || tb.Rows[1][1] != "80%" {
		t.Errorf("table %+v", bs[12].Table)
	}
	if sp := bs[7].Spans; len(sp) != 3 || sp[1].Href != "#tab-1" || sp[2].Href != "fn-1" {
		t.Errorf("spans %+v", sp)
	}
	if bs[14].Image == nil || bs[14].Image.ID != "img-1" {
		t.Errorf("image %+v", bs[14].Image)
	}
	st := doc.Parts[len(doc.Parts)-1]
	found := false
	for _, b := range st.Blocks {
		if b.Attrs["cda.id"] == "prob1" && b.Text == "Asthma" {
			found = true
		}
	}
	if !found {
		t.Errorf("content ID record missing: %s", dump(doc))
	}
}

func TestCDACorpus(t *testing.T) {
	// The one dev file not selected is a stub whose ClinicalDocument has
	// no namespace (not CDA: CDA's elements are in urn:hl7-org:v3); it is
	// read generically.
	patentRunCorpus(t, "xml-cda", "cda", "Referral_Request_aanned_Referral")
}

// cdaOracleSkip lists the elements whose character data is metadata or
// stored, not block text, by design.
var cdaOracleSkip = map[string]string{
	"/recordTarget": "metadata", "/author": "metadata", "/dataEnterer": "metadata", "/informant": "metadata",
	"/custodian": "metadata", "/informationRecipient": "metadata", "/legalAuthenticator": "metadata",
	"/authenticator": "metadata", "/participant": "metadata", "/inFulfillmentOf": "metadata",
	"/documentationOf": "metadata", "/relatedDocument": "metadata", "/componentOf": "metadata",
	"nonXMLBody/text": "stored",
}

func TestCDAOracles(t *testing.T) {
	patentRunOracles(t, "xml-cda", cdaOracleSkip)
}

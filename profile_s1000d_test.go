package gofastxml

import (
	"strings"
	"testing"
)

// The corpus, oracle and fixture helpers are in profile_patent_test.go.

func TestS1000DFixture(t *testing.T) {
	_, doc := patentFixture(t, "s1000d/dm.xml", "s1000d")
	patentExtra(t, doc, map[string]string{
		"s1000d.module": "dmodule",
		"s1000d.ident": "dmCode(modelIdentCode=TESTBIKE, systemDiffCode=A, systemCode=DA0, subSystemCode=1, subSubSystemCode=0, assyCode=00, disassyCode=00, disassyCodeVariant=AA, infoCode=520, infoCodeVariant=A, itemLocationCode=A), " +
			"language(countryIsoCode=US, languageIsoCode=en), issueInfo(issueNumber=002, inWork=00)",
		"s1000d.issue_year":          "2026",
		"s1000d.issue_month":         "01",
		"s1000d.issue_day":           "31",
		"s1000d.status":              "new",
		"s1000d.security_class":      "01",
		"s1000d.responsible_partner": "Example Co", "s1000d.responsible_partner_code": "U0001",
		"s1000d.originator": "Example Co", "s1000d.applic": "All bikes",
		"s1000d.quality_assurance": "unverified",
		"s1000d.brex":              "dmRef(dmRefIdent(dmCode(modelIdentCode=S1000D, systemDiffCode=F, systemCode=04, subSystemCode=1, subSubSystemCode=0, assyCode=0301, disassyCode=00, disassyCodeVariant=A, infoCode=022, infoCodeVariant=A, itemLocationCode=D)))",
		"dc:date":                  "2026-01-31",
	})
	if doc.Metadata.Title != "Front wheel Remove procedures" || len(doc.Metadata.Authors) != 1 {
		t.Errorf("title %q authors %q", doc.Metadata.Title, doc.Metadata.Authors)
	}
	patentCheck(t, doc, `heading1:Front wheel Remove procedures
paragraph:Model A
list_item1:The bicycle is clean
list_item1:Tool set K1 T-1 1
paragraph:Wear gloves.
paragraph:Hold the front of the bicycle (refer to Fig 1).
paragraph:Lift the wheel, see Front wheel Install.
paragraph:Keep the axle.
heading2:Data
paragraph:Values:
caption:Front wheel
image:
caption:Torques
table:
`, `heading #
paragraph #app-1 s1000d.applic_evaluate=assert(applicPropertyIdent=model, applicPropertyType=prodattr, applicPropertyValues=A), assert(applicPropertyIdent=model, applicPropertyType=prodattr, applicPropertyValues=B) s1000d.part=referencedApplicGroup
list_item # s1000d.content=procedure s1000d.part=preliminaryRqmts s1000d.req=reqCondGroup
list_item #seq-1 s1000d.content=procedure s1000d.part=preliminaryRqmts s1000d.req=reqSupportEquips
paragraph # anchor_parent=stp-1 s1000d.applic_ref=app-1 s1000d.content=procedure s1000d.kind=warning s1000d.part=mainProcedure s1000d.step=stp-1
paragraph # anchor_parent=stp-1 s1000d.applic_ref=app-1 s1000d.content=procedure s1000d.part=mainProcedure s1000d.step=stp-1
paragraph # anchor_parent=stp-2 s1000d.content=procedure s1000d.part=mainProcedure s1000d.step=stp-2
paragraph # anchor_parent=stp-2 s1000d.content=procedure s1000d.kind=note s1000d.part=mainProcedure s1000d.step=stp-2
heading # anchor_parent=par-1
paragraph #p-1
caption # anchor_parent=fig-1
image #gra-1
caption # anchor_parent=tab-1
table #tab-1
`)
	bs := blocks(doc)
	if sp := bs[5].Spans; len(sp) != 2 || sp[1].Href != "fig-1" {
		t.Errorf("internalRef spans %+v", sp)
	}
	if sp := bs[6].Spans; len(sp) != 1 || !strings.HasPrefix(sp[0].Href, "dmCode(modelIdentCode=TESTBIKE") || !strings.Contains(sp[0].Href, "infoCode=720") {
		t.Errorf("dmRef spans %+v", sp)
	}
	if bs[11].Image == nil || bs[11].Image.ID != "ICN-TEST-00001-A-01" {
		t.Errorf("graphic %+v", bs[11].Image)
	}
}

func TestS1000DCorpus(t *testing.T) {
	patentRunCorpus(t, "xml-s1000d", "s1000d", "")
}

// s1000dOracleSkip lists the elements whose character data is metadata,
// not block text, by design.
var s1000dOracleSkip = map[string]string{
	"dmStatus": "metadata", "pmStatus": "metadata", "dmlStatus": "metadata",
	"Description": "metadata (Dublin Core)", "pmAddressItems/shortPmTitle": "metadata",
}

func TestS1000DOracles(t *testing.T) {
	patentRunOracles(t, "xml-s1000d", s1000dOracleSkip)
}

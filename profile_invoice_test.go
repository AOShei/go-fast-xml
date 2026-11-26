package gofastxml

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AOShei/go-fast-core"
)

// invoiceFixture reads testdata/profiles/<name> and checks what every
// fixture must hold: no error, Validate, every block cited.
func invoiceFixture(t *testing.T, name, profile string, opts *gofastcore.Options) ([]byte, *gofastcore.Document) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "profiles", name))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ExtractBytes(context.Background(), data, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if bad := citeCheck(data, doc); bad != "" {
		t.Errorf("cite: %s", bad)
	}
	if got := doc.Metadata.Extra[ExtraProfile]; got != profile {
		t.Fatalf("profile %q, want %q", got, profile)
	}
	return data, doc
}

// invoiceTable returns the table whose header row starts with first, the
// n-th such (0-based).
func invoiceTable(t *testing.T, doc *gofastcore.Document, first string, n int) *gofastcore.Block {
	t.Helper()
	for _, p := range doc.Parts {
		for i := range p.Blocks {
			b := &p.Blocks[i]
			if b.Table != nil && len(b.Table.Rows) > 0 && b.Table.Rows[0][0] == first {
				if n == 0 {
					return b
				}
				n--
			}
		}
	}
	t.Fatalf("no table %q", first)
	return nil
}

// invoicePara returns the text of the first paragraph whose Attrs hold
// every key and value of want.
func invoicePara(doc *gofastcore.Document, want map[string]string) (string, bool) {
	for _, p := range doc.Parts {
	blocks:
		for _, b := range p.Blocks {
			if b.Kind != gofastcore.KindParagraph {
				continue
			}
			for k, v := range want {
				if b.Attrs[k] != v {
					continue blocks
				}
			}
			return b.Text, true
		}
	}
	return "", false
}

func invoiceRowLocs(t *testing.T, data []byte, b *gofastcore.Block, prefix string) {
	t.Helper()
	tb := b.Table
	if len(tb.RowLocs) != len(tb.Rows) {
		t.Fatalf("%d RowLocs for %d rows", len(tb.RowLocs), len(tb.Rows))
	}
	for i, l := range tb.RowLocs[1:] {
		src := string(data[l.Offset : l.Offset+l.Length])
		if !strings.HasPrefix(src, prefix) {
			t.Errorf("row %d cites %.40q, want %q", i+1, src, prefix)
		}
	}
	if tb.RowLocs[0] != (gofastcore.Location{}) || b.Attrs[AttrHeaderSource] != "profile" || tb.HeaderRows != 1 {
		t.Errorf("header row: loc %+v, attrs %v, HeaderRows %d", tb.RowLocs[0], b.Attrs, tb.HeaderRows)
	}
}

// invoiceRows checks a table's rows.
func invoiceRows(t *testing.T, b *gofastcore.Block, want [][]string) {
	t.Helper()
	if !slices.EqualFunc(b.Table.Rows, want, slices.Equal) {
		t.Errorf("rows:\n%q\nwant\n%q", b.Table.Rows, want)
	}
}

func TestInvoiceCII(t *testing.T) {
	data, doc := invoiceFixture(t, "cii/invoice.xml", "cii", nil)
	patentExtra(t, doc, map[string]string{
		"inv.number": "F-2026-001", "inv.type_code": "380", "inv.currency": "EUR",
		"inv.guideline_id": "urn:cen.eu:en16931:2017", "inv.issue_date_format": "102",
	})
	if want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC); !doc.Metadata.Created.Equal(want) {
		t.Errorf("Created %v", doc.Metadata.Created)
	}
	lines := invoiceTable(t, doc, "line id", 0)
	invoiceRowLocs(t, data, lines, "<ram:IncludedSupplyChainTradeLineItem>")
	want := [][]string{
		{"line id", "item name", "description", "quantity", "net price", "line net amount", "VAT category", "VAT rate", "note", "item identifiers", "gross price", "item attributes", "billing period"},
		{"1", "Papier A4", "Ramette 500 feuilles", "10 C62", "4.50", "45.00", "S", "20", "Livraison partielle", "4012345000009 0160", "", "Grammage\n80 g", ""},
		{"2", "Stylo", "", "3 H87", "1.00", "3.00", "S", "20", "", "", "", "", ""},
	}
	if !slices.EqualFunc(lines.Table.Rows, want, slices.Equal) {
		t.Errorf("lines:\n%q\nwant\n%q", lines.Table.Rows, want)
	}
	// What the lines hold beyond their columns: after the lines table,
	// with the line's id and the file's name for each value.
	rest := invoiceTable(t, doc, "line id", 1)
	invoiceRowLocs(t, data, rest, "<ram:")
	invoiceRows(t, rest, [][]string{{"line id", "name", "value"},
		{"1", "SpecifiedTradeProduct/OriginTradeCountry/ID", "FR"},
		{"1", "SpecifiedLineTradeSettlement/ApplicableTradeTax/TypeCode", "VAT"},
		{"2", "SpecifiedLineTradeSettlement/ApplicableTradeTax/TypeCode", "VAT"}})
	tax := invoiceTable(t, doc, "tax type", 0)
	if got := tax.Table.Rows[1]; !slices.Equal(got, []string{"VAT", "S", "20", "48.00", "9.60", "", ""}) {
		t.Errorf("VAT breakdown %q", got)
	}
	ac := invoiceTable(t, doc, "charge indicator", 0)
	if got := ac.Table.Rows[1]; !slices.Equal(got, []string{"true", "0.00", "", "", "", "Port", "S", "20"}) {
		t.Errorf("allowance/charge %q", got)
	}
	tot := invoiceTable(t, doc, "line total", 0)
	if got := tot.Table.Rows[1]; !slices.Equal(got, []string{"48.00", "", "", "48.00", "9.60 EUR", "", "57.60", "", "57.60"}) {
		t.Errorf("totals %q", got)
	}
	nv := []string{"name", "value"}
	for i, c := range []struct {
		attrs map[string]string
		rows  [][]string
	}{
		{map[string]string{"inv.party": "seller"}, [][]string{nv, {"SellerTradeParty/Name", "Exemple SARL"},
			{"SellerTradeParty/SpecifiedLegalOrganization/ID", "123456789"}, {"SellerTradeParty/SpecifiedLegalOrganization/ID/@schemeID", "0002"},
			{"SellerTradeParty/PostalTradeAddress/PostcodeCode", "75001"}, {"SellerTradeParty/PostalTradeAddress/CityName", "Paris"}, {"SellerTradeParty/PostalTradeAddress/CountryID", "FR"},
			{"SellerTradeParty/SpecifiedTaxRegistration/ID", "FR32123456789"}, {"SellerTradeParty/SpecifiedTaxRegistration/ID/@schemeID", "VA"}}},
		{map[string]string{"inv.party": "buyer"}, [][]string{nv, {"BuyerTradeParty/Name", "Client SA"}}},
		{map[string]string{"inv.group": "SpecifiedTradeSettlementPaymentMeans"}, [][]string{nv, {"SpecifiedTradeSettlementPaymentMeans/TypeCode", "58"},
			{"SpecifiedTradeSettlementPaymentMeans/PayeePartyCreditorFinancialAccount/IBANID", "FR7630006000011234567890189"}}},
		{nil, [][]string{nv, {"CategoryTradeTax/TypeCode", "VAT"}}},
		{map[string]string{"inv.group": "SpecifiedTradePaymentTerms"}, [][]string{nv, {"SpecifiedTradePaymentTerms/Description", "30 jours net"},
			{"SpecifiedTradePaymentTerms/DueDateDateTime/DateTimeString", "20260214"}, {"SpecifiedTradePaymentTerms/DueDateDateTime/DateTimeString/@format", "102"}}},
	} {
		b := invoiceTable(t, doc, "name", i)
		invoiceRowLocs(t, data, b, "<")
		invoiceRows(t, b, c.rows)
		for k, v := range c.attrs {
			if b.Attrs[k] != v {
				t.Errorf("table %d: Attrs %v, want %s=%s", i, b.Attrs, k, v)
			}
		}
	}
	if got, ok := invoicePara(doc, map[string]string{"inv.field": "IncludedNote", "inv.subject_code": "AAI"}); !ok || got != "Merci de votre confiance." {
		t.Errorf("note %q, %v", got, ok)
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes || w.Code == WarnUnmappedElements {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
}

func TestInvoiceUBL(t *testing.T) {
	data, doc := invoiceFixture(t, "ubl/invoice.xml", "ubl", nil)
	patentExtra(t, doc, map[string]string{
		"inv.number": "INV-7", "inv.type_code": "380", "inv.currency": "EUR",
		"inv.customization_id": "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0",
		"inv.profile_id":       "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0",
	})
	if want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC); !doc.Metadata.Created.Equal(want) {
		t.Errorf("Created %v", doc.Metadata.Created)
	}
	lines := invoiceTable(t, doc, "line id", 0)
	invoiceRowLocs(t, data, lines, "<cac:InvoiceLine>")
	want := []string{"1", "Consulting", "On-site work", "4 HUR", "25.00 EUR", "100.00 EUR", "S", "20", "Rush order", "SRV-1", "72224000 STI", "", ""}
	if got := lines.Table.Rows[1]; !slices.Equal(got, want) {
		t.Errorf("line:\n%q\nwant\n%q", got, want)
	}
	rest := invoiceTable(t, doc, "line id", 1)
	invoiceRowLocs(t, data, rest, "<cbc:")
	invoiceRows(t, rest, [][]string{{"line id", "name", "value"}, {"1", "AccountingCost", "CC-9"}, {"1", "Item/ClassifiedTaxCategory/TaxScheme/ID", "VAT"}})
	tax := invoiceTable(t, doc, "taxable amount", 0)
	invoiceRowLocs(t, data, tax, "<cac:TaxSubtotal>")
	if got := tax.Table.Rows[1]; !slices.Equal(got, []string{"95.00 EUR", "19.00 EUR", "S", "20", "", "", "VAT"}) {
		t.Errorf("tax subtotal %q", got)
	}
	ac := invoiceTable(t, doc, "charge indicator", 0)
	if got := ac.Table.Rows[1]; !slices.Equal(got, []string{"false", "5.00 EUR", "", "", "", "Discount", "S", "20"}) {
		t.Errorf("allowance/charge %q", got)
	}
	tot := invoiceTable(t, doc, "line extension amount", 0)
	if got := tot.Table.Rows[1]; !slices.Equal(got, []string{"100.00 EUR", "95.00 EUR", "114.00 EUR", "", "", "", "", "114.00 EUR"}) {
		t.Errorf("totals %q", got)
	}
	if got, ok := invoicePara(doc, map[string]string{"inv.field": "Note"}); !ok || got != "Thank you." {
		t.Errorf("note %q, %v", got, ok)
	}
	// Values outside a party or a group are tables of their own; the
	// second TaxTotal has no subtotal: its amount is such a table and no
	// header-only table is made.
	nv := []string{"name", "value"}
	for i, c := range []struct {
		attrs map[string]string
		rows  [][]string
	}{
		{nil, [][]string{nv, {"DueDate", "2026-02-14"}}},
		{map[string]string{"inv.party": "seller"}, [][]string{nv, {"AccountingSupplierParty/Party/EndpointID", "7300010000001"}, {"AccountingSupplierParty/Party/EndpointID/@schemeID", "0088"},
			{"AccountingSupplierParty/Party/PartyName/Name", "Seller Ltd"}, {"AccountingSupplierParty/Party/PostalAddress/CityName", "London"}, {"AccountingSupplierParty/Party/PostalAddress/Country/IdentificationCode", "GB"},
			{"AccountingSupplierParty/Party/PartyTaxScheme/CompanyID", "GB123456789"}, {"AccountingSupplierParty/Party/PartyTaxScheme/TaxScheme/ID", "VAT"}}},
		{map[string]string{"inv.party": "buyer"}, [][]string{nv, {"AccountingCustomerParty/Party/PartyName/Name", "Buyer plc"}}},
		{map[string]string{"inv.group": "PaymentMeans"}, [][]string{nv, {"PaymentMeans/PaymentMeansCode", "30"}, {"PaymentMeans/PaymentMeansCode/@name", "Credit transfer"},
			{"PaymentMeans/PayeeFinancialAccount/ID", "GB29NWBK60161331926819"}}},
		{nil, [][]string{nv, {"TaxCategory/TaxScheme/ID", "VAT"}}},
		{nil, [][]string{nv, {"TaxAmount", "19.00"}, {"TaxAmount/@currencyID", "EUR"}}},
		{nil, [][]string{nv, {"TaxAmount", "16.40"}, {"TaxAmount/@currencyID", "GBP"}}},
	} {
		b := invoiceTable(t, doc, "name", i)
		invoiceRowLocs(t, data, b, "<c")
		invoiceRows(t, b, c.rows)
		for k, v := range c.attrs {
			if b.Attrs[k] != v {
				t.Errorf("table %d: Attrs %v, want %s=%s", i, b.Attrs, k, v)
			}
		}
	}
	for _, w := range doc.AllWarnings() {
		if w.Code == WarnUnmappedAttributes || w.Code == WarnUnmappedElements {
			t.Errorf("warning %s: %s", w.Code, w.Message)
		}
	}
}

// TestInvoiceCorpus reads the xml-invoice corpus: dev files with a CII or
// UBL Invoice/CreditNote root must be read with cii or ubl, the others
// (ZUGFeRD 1.0 CrossIndustryDocument, other roots) generically. Held-out
// files are counted only.
func TestInvoiceCorpus(t *testing.T) {
	want := map[string]string{
		"urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100": "cii",
		"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2":       "ubl",
		"urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2":    "ubl",
	}
	for _, set := range []string{"dev", "heldout"} {
		dir := patentCorpusDir(set, "xml-invoice")
		if dir == "" {
			t.Logf("%s: no xml-invoice corpus", set)
			continue
		}
		tally := newTally()
		profiles := map[string]int{}
		codes := map[string]int{}
		unmapped := map[string]int{}
		for _, path := range patentCorpusFiles(dir) {
			res := corpusOne(path)
			tally.add("xml-invoice", res)
			label := path
			if set == "heldout" {
				label = "a held-out file"
			}
			switch {
			case res.panic != "":
				t.Errorf("%s: panic: %s", label, firstLine(res.panic, set))
			case res.timeout:
				t.Errorf("%s: no result within the time limit", label)
			case res.err != nil && errors.Is(res.err, gofastcore.ErrUnsupported) && set == "dev":
				// One dev file declares encoding="utf-8-w/o-bom" (a deliberate
				// test case): an encoding label this library does not know is
				// ErrUnsupported, as the README says, whatever the profile.
				t.Logf("%s: %v", label, res.err)
			case res.err != nil:
				t.Errorf("%s: error: %s", label, firstLine(res.err.Error(), set))
			case res.invalid != nil:
				t.Errorf("%s: Validate: %s", label, firstLine(res.invalid.Error(), set))
			}
			if res.doc == nil {
				continue
			}
			got := res.doc.Metadata.Extra[ExtraProfile]
			profiles[got]++
			for _, w := range res.doc.AllWarnings() {
				codes[w.Code]++
				if set == "dev" && got != "" && (w.Code == WarnUnmappedElements || w.Code == WarnUnmappedAttributes) {
					for _, sm := range countRE.FindAllStringSubmatch(w.Message, -1) {
						n, _ := strconv.Atoi(sm[2])
						unmapped[got+" "+strings.TrimSpace(sm[1])] += n
					}
				}
			}
			if set == "dev" {
				exp := want[res.doc.Metadata.Extra["root_namespace"]]
				if exp != "" && got != exp {
					t.Errorf("%s: read with profile %q, want %q", label, got, exp)
				}
			}
		}
		t.Logf("%s: profiles used: %s; warnings: %s\n%s", set, byCount(profiles), byCount(codes), tally)
		if set == "dev" {
			t.Logf("dev: unmapped under cii/ubl: %s", byCount(unmapped))
		}
	}
}

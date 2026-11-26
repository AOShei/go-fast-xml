package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AOShei/go-fast-core"
)

// ciiAttached is a CII invoice with two attachments: an HTML page (base64
// of "<p>Hello</p>") and a PDF the command has no extractor for.
const ciiAttached = `<?xml version="1.0"?>
<rsm:CrossIndustryInvoice xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100" xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100">
<rsm:ExchangedDocument><ram:ID>A-1</ram:ID></rsm:ExchangedDocument>
<rsm:SupplyChainTradeTransaction><ram:ApplicableHeaderTradeAgreement>
<ram:AdditionalReferencedDocument><ram:IssuerAssignedID>d1</ram:IssuerAssignedID><ram:AttachmentBinaryObject mimeCode="text/html" filename="page.html">PHA+SGVsbG88L3A+</ram:AttachmentBinaryObject></ram:AdditionalReferencedDocument>
<ram:AdditionalReferencedDocument><ram:IssuerAssignedID>d2</ram:IssuerAssignedID><ram:AttachmentBinaryObject mimeCode="application/pdf" filename="a.pdf">JVBERg==</ram:AttachmentBinaryObject></ram:AdditionalReferencedDocument>
</ram:ApplicableHeaderTradeAgreement></rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>`

// TestInvoiceAttachments: the HTML attachment is extracted by go-fast-html,
// the PDF is listed as unsupported, and no base64 is in the body.
func TestInvoiceAttachments(t *testing.T) {
	a := filepath.Join(t.TempDir(), "invoice.xml")
	os.WriteFile(a, []byte(ciiAttached), 0o644)
	var out, errs bytes.Buffer
	if code := run(context.Background(), []string{a}, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	var doc gofastcore.Document
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(doc.Children) != 2 {
		t.Fatalf("children %d", len(doc.Children))
	}
	h, p := doc.Children[0], doc.Children[1]
	if h.Source.Name != "page.html" || len(h.Parts) == 0 || len(h.Parts[0].Blocks) == 0 || h.Parts[0].Blocks[0].Text != "Hello" {
		t.Errorf("html child %+v", h)
	}
	unsupported := false
	for _, w := range p.Warnings {
		unsupported = unsupported || w.Code == gofastcore.WarnChildUnsupported
	}
	if p.Source.Name != "a.pdf" || !unsupported {
		t.Errorf("pdf child %+v", p)
	}
	if s := out.String(); strings.Contains(s, "PHA+") || strings.Contains(s, "JVBERg") {
		t.Errorf("base64 in the output")
	}
}

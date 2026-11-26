# Profile `ubl`: OASIS UBL 2.x Invoice and CreditNote (Peppol BIS Billing 3.0)

**Chosen for:** roots `Invoice`
(`urn:oasis:names:specification:ubl:schema:xsd:Invoice-2`) and `CreditNote`
(`...:CreditNote-2`) only. Other UBL documents (orders, despatch advices,
eForms notices) stay generic. The `cac:`/`cbc:` rules for parties,
addresses, references, payment and tax are not tied to the root, so that a
later profile (eForms, item 7 of the note) can reuse them.

## Mapping

| Part | Where it goes |
|---|---|
| `cbc:ID`, `IssueDate`, `InvoiceTypeCode`/`CreditNoteTypeCode`, `DocumentCurrencyCode` | `Extra["inv.number"]`, `Metadata.Created` (ISO date), `Extra["inv.type_code"]` (with `inv.type_code_list`), `Extra["inv.currency"]` |
| `CustomizationID`, `ProfileID`, `UBLVersionID`, `TaxCurrencyCode`, `IssueTime`, `UUID` | `Extra["inv.customization_id"]`, `inv.profile_id`, `inv.ubl_version`, `inv.tax_currency`, `inv.issue_time`, `inv.uuid` |
| `cbc:Note` | a paragraph with `inv.field=Note` (`inv.languageID`) |
| `BuyerReference`, `AccountingCost`, `DueDate`, `TaxPointDate` and the other single values | a table of names and values of its own (`pairs`): its local name, its text; its attributes as rows of their own |
| parties (`AccountingSupplierParty`, `AccountingCustomerParty`, `PayeeParty`, `TaxRepresentativeParty`, `Delivery`, ...) | a table of names and values (`pairs`), no heading, carrying `inv.party` (`seller`, `buyer`, `payee`, `tax_representative`, `delivery`) on the table block. A row per value: its path of local names below the party element (`Party/EndpointID`, `Party/PartyName/Name`, `Party/PostalAddress/Country/IdentificationCode`, `Party/PartyTaxScheme/CompanyID`), its text; each attribute a row after its element's (`Party/EndpointID/@schemeID`) |
| `PaymentMeans`, `PaymentTerms`, document references, `InvoicePeriod` | a table of names and values each, carrying `inv.group`: `PaymentMeansCode`, `PaymentMeansCode/@name`, `PayeeFinancialAccount/ID` (the IBAN), `PayeeFinancialAccount/FinancialInstitutionBranch/ID` (the BIC) |
| lines (`cac:InvoiceLine`, `cac:CreditNoteLine`) | one table, a row per line with `RowLocs`; then the line values table (below) |
| `cac:TaxTotal` | a table, a row per `TaxSubtotal` (`RowLocs`), then a table of VAT category (the subtotal's column, as written), name and value for what a subtotal holds beyond its columns; the total `TaxAmount` is a table of names and values of its own (`TaxAmount`, `TaxAmount/@currencyID`). A `TaxTotal` with no subtotal (the tax currency's) makes only that table |
| `cac:AllowanceCharge` (document level) | a one-row table each; then a table of names and values for the rest (`TaxCategory/TaxScheme/ID`, ...) |
| `cac:LegalMonetaryTotal` | a one-row table; then a table of names and values for the rest |
| `cac:Attachment/EmbeddedDocumentBinaryObject` | an attachment: decoded from base64 and handed to `Options.ExtractChild` (`"encoding": "base64"`), named by `@filename`, media type `@mimeCode` as written (`application/octet-stream` when absent). The base64 is not in the body; a child not extracted is still listed in `Document.Children`, with `xml.parent_offset`/`length`/`element`, and the table of its `AdditionalDocumentReference` carries `Attrs["children"]`. Text that is not base64 stays as a paragraph with `media_type` and `input.stream_damaged`, before that table. `cac:ExternalReference/cbc:URI` is a row as written, never fetched |

**Names.** As in `cii` (see `profiles/cii.md`): each value of a table of
names and values is written with the file's path of local names below the
party, group or line, never translated; the header row is the profile's.
UBL wraps a party in `cac:Party`, so party names start with `Party/`, as
the file writes them.

**The lines table.** The root element is the table (UBL has no element
around the lines); everything else in the document is read as blocks made
before it. Columns (the profile's header row): line id, item name,
description, quantity (`unitCode`), net price (`PriceAmount` with
`currencyID`, then `BaseQuantity`), line net amount, VAT category, VAT
rate, note, item identifiers (seller's, buyer's, standard with
`schemeID`), item classification (`ItemClassificationCode` with `listID`
and `listVersionID`), item attributes (`AdditionalItemProperty`: name,
value), billing period. Everything else in a line (accounting cost, order
line reference, document references, line allowances and charges, price
allowance, origin country, tax scheme) is a row of the **line values
table** right after it: line id (as written in the line's first column),
name (`AccountingCost`, `Item/ClassifiedTaxCategory/TaxScheme/ID`), value,
in file order, `RowLocs` on the value's element. There is no `other`
column.

**Amounts** keep `currencyID` after the value in cells (`25.00 EUR`); in
a table of names and values `currencyID` is a row of its own.

## Corpus (dev)

82 files (75 `Invoice`, 7 `CreditNote`) read with `ubl`, no error, no
`Validate` failure, no element left generic; the body's only paragraphs
are the `cbc:Note`s (79). The figures for the whole xml-invoice set
(both profiles) are in `profiles/cii.md`.

**Oracles** (`TestInvoiceOracles`, dev, with and without a child
extractor): (a) every character data run is found exactly once, in its
row's cells when it is inside a row element, else outside tables in file
order; the exceptions are tested and each is met: the header values that
go to `Metadata` (`/Invoice/ID`, `IssueDate`, `InvoiceTypeCode`,
`DocumentCurrencyCode`, `CustomizationID`, `ProfileID`, `TaxCurrencyCode`,
and the `CreditNote` ones), the attachments (handed over), the profile's
header rows, and the names (and line ids) of the tables of names and
values, each checked against the element its row cites (see
`profiles/cii.md`); attribute values written into cells (`with`) are
matched after their element's text. (b) every block's text is in the
source at its offsets, every row's `RowLocs` lie in its table and in file
order, and each word of a row's cells is in the source at its `RowLocs`.
All pass.

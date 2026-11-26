# Profile `cii`: UN/CEFACT Cross Industry Invoice (Factur-X, ZUGFeRD 2, XRechnung CII)

**Chosen for:** root `CrossIndustryInvoice` in
`urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100` (D16B, the
syntax EN 16931 binds). The guideline (MINIMUM, BASIC WL, BASIC, EN 16931,
EXTENDED, XRECHNUNG) is `Metadata.Extra["inv.guideline_id"]`, as written.

**Not chosen for:** ZUGFeRD 1.0 (`CrossIndustryDocument` in
`urn:ferd:CrossIndustryDocument:invoice:1p0`, `ram` namespace `...:12`).
Its elements have other names and another structure
(`HeaderExchangedDocument`, `SpecifiedSupplyChainTradeTransaction`,
`ApplicableSupplyChainTradeAgreement`...), so the rules cannot be shared by
a namespace alias; it is read generically (16 of 223 dev files).

## Mapping

| Part | Where it goes |
|---|---|
| `ExchangedDocument/ID`, `TypeCode`, `Name` | `Extra["inv.number"]` (`inv.number_scheme`), `Extra["inv.type_code"]`, `Metadata.Title` |
| `IssueDateTime/DateTimeString` | `Metadata.Created` when `format="102"` (`YYYYMMDD`), else `Extra["inv.issue_date"]` as written; `Extra["inv.issue_date_format"]` |
| guideline, business process, test indicator, `InvoiceCurrencyCode`, `TaxCurrencyCode` | `Extra["inv.guideline_id"]`, `inv.business_process_id`, `inv.test_indicator`, `inv.currency`, `inv.tax_currency` |
| `IncludedNote/Content` | a paragraph, `inv.field=IncludedNote`, with `inv.subject_code`/`inv.content_code` |
| parties (`SellerTradeParty`, `BuyerTradeParty`, `PayeeTradeParty`, `SellerTaxRepresentativeTradeParty`, `ShipToTradeParty`, `InvoiceeTradeParty` and the other `*TradeParty`) | a table of names and values (`pairs`), no heading, carrying `inv.party` (`seller`, `buyer`, `payee`, `seller_tax_representative`, `ship_to`, ...) on the table block. A row per value: its path of local names from the party's element on (`SellerTradeParty/Name`, `SellerTradeParty/PostalTradeAddress/CityName`, `BuyerTradeParty/GlobalID`), its text; each attribute a row of its own after its element's (`SellerTradeParty/SpecifiedTaxRegistration/ID/@schemeID`). The party's own element name is in every row, so rendered text tells the seller's table from the buyer's |
| references (`BuyerOrderReferencedDocument`, `AdditionalReferencedDocument`, ...), delivery event, billing period, payment means, payment terms, logistics charges, currency exchange, accounting account, project | a table of names and values each, carrying `inv.group` (the element's name): `SpecifiedTradeSettlementPaymentMeans/TypeCode`, `SpecifiedTradeSettlementPaymentMeans/PayeePartyCreditorFinancialAccount/IBANID`, `SpecifiedTradePaymentTerms/DueDateDateTime/DateTimeString` with `.../@format`, ... |
| a value outside these (`BuyerReference`, `PaymentReference`, `CreditorReferenceID`, ...) | a table of its own of one row (and its attributes): its local name, its text |
| lines (`IncludedSupplyChainTradeLineItem`) | one table, a row per line with `RowLocs`; then the line values table (below) |
| `ApplicableTradeTax` (header) | a one-row table each: tax type, VAT category, rate, basis, tax amount, exemption reason and code; then, if it holds more (`DueDateTypeCode`, `TaxPointDate`, ...), a table of names and values |
| `SpecifiedTradeAllowanceCharge` (header) | a one-row table each; then a table of names and values for the rest (`CategoryTradeTax/TypeCode`, ...) |
| `SpecifiedTradeSettlementHeaderMonetarySummation` | a one-row table; then a table of names and values for the rest |
| `AdditionalReferencedDocument/AttachmentBinaryObject` | an attachment: decoded from base64 and handed to `Options.ExtractChild` (`"encoding": "base64"`), named by `@filename`, media type `@mimeCode` as written (`application/octet-stream` when absent). The base64 is not in the body; a child not extracted is still listed in `Document.Children` with `xml.parent_offset`/`length`/`element`, and the table of its `AdditionalReferencedDocument` carries `Attrs["children"]`. Text that is not base64 stays as a paragraph with `media_type` and `input.stream_damaged`, before that table |

**Names.** A table of names and values (profile keys `pairs` and
`rest_table`, `PROFILE_FORMAT.md`) writes each value with the file's name
for it, as written, never translated: the path of local names starting
with the party's or group's own element (`pairs`), or below the line's
element (`rest_table`, where the line's id is the first cell); `/@` and the attribute's name for an attribute.
Its header row (`name`, `value`, or `line id`, `name`, `value`) is the
profile's (`header_source=profile`). Each row's `RowLocs` is the value's
element. Rules for the elements inside are not applied (an attachment's
is), so a value's codes and amounts keep their attributes as rows of
their own, not as `inv.*` Attrs.

**The lines table.** CII has no element around the lines, so
`SupplyChainTradeTransaction` is the table; the header agreement, delivery
and settlement it also holds are read as blocks (made before the table
block, which is made when the transaction ends). The table's header row is
the profile's (`header_source=profile`): line id, item name, description,
quantity (with `unitCode`), net price (`ChargeAmount`, then `BasisQuantity`
with `unitCode`), line net amount, VAT category, VAT rate, note, item
identifiers (seller's, buyer's, global with `schemeID`), gross price, item
attributes (`ApplicableProductCharacteristic`: description, value), billing
period. Everything else a line holds (origin country, tax type, exemption
reason, line allowances and charges, references, accounting account) is a
row of the **line values table** right after it: line id (the text of the
line's first column, as written), name (path below the line element:
`SpecifiedTradeProduct/OriginTradeCountry/ID`), value, in file order, with
`RowLocs` on the value's element. There is no `other` column.

**Amounts** are as written, with `currencyID` after the value in the cell
text (`9.60 EUR`), through the profile key `with`; CII writes currency on
few amounts (`TaxTotalAmount`), the invoice currency being
`inv.currency`. In a table of names and values, `currencyID` is a row.

CII has no element around the allowances or the VAT breakdown either, so
each is a table of one row (`one_row`), not one table for all.

## Corpus (dev, 223 files)

123 CII files read with `cii`; 82 UBL with `ubl`; 17 generic (16 ZUGFeRD
1.0, 1 `CBFBUY` invoice). One file declares `encoding="utf-8-w/o-bom"` and
is `ErrUnsupported` (the README's rule for unknown encoding labels).

Measured by `TestInvoiceCorpus` and `TestInvoiceOracles` at this commit:

| | dev (223) | held-out (91) |
|---|---|---|
| read with `cii` / `ubl` / generic | 123 / 82 / 17 | 56 / 28 / 7 |
| errors | 1 (`ErrUnsupported`, the encoding label) | 0 |
| `Validate` failures, panics | 0 | 0 |
| blocks (all files) | table 2700, paragraph 1784, note attribute 546, stored `markup_comment` 892 | table 1208, paragraph 748, note attribute 222, stored 356 |
| warnings (documents with) | `xml.unmapped_elements` 2 | `xml.unmapped_elements` 1, `xml.unmapped_attributes` 1 |
| attachments handed over (cii and ubl) | 29 | not counted |
| names checked by the oracle (cii and ubl) | 8363 | not run |

Under `cii` and `ubl` on dev, the only paragraphs of the body are the
notes (`IncludedNote/Content` 240, `cbc:Note` 79) and the two generic
`BuyerReference` below; every other value is in a table with its name.
The other paragraphs counted above are those of the 17 generic files.

**Left generic on dev:** `BuyerReference` (2, in two files that write it
in no namespace, outside the `ram` namespace the rules name): its text is
an inferred paragraph at its place. Nothing else under `cii` or `ubl`.

**Depth in a line.** A value in the line values table may be at any
depth below the line. Inside a column cell (item attributes, billing
period) the rules go five levels below the line element; on dev,
character data below a line is at depth 1 (754 runs), 2 (2077), 3 (2825),
4 (295) and 5 (27), never deeper.

**Oracles** (`TestInvoiceOracles`, dev, with and without a child
extractor): (a) every character data run is found exactly once, in its
row's cells when it is inside a row element, else outside tables in file
order; the exceptions are tested and each is met: the header values that
go to `Metadata` (`ExchangedDocument/ID`, `TypeCode`, `Name`,
`IssueDateTime`, guideline and business process `ID`, `TestIndicator`,
`InvoiceCurrencyCode`, `TaxCurrencyCode`), the note codes that go to
`Attrs` (`IncludedNote/SubjectCode`, `ContentCode`), the attachments
(handed over), the profile's header rows, and the names of a table of
names and values with its line ids: each name is checked to be the end of
the path of local names of the element its row cites (an `/@` name: an
attribute of that element whose value is the row's value), each line id
the text of a cell of the line row holding that element (8363 names on
dev); attribute values written into cells (`with`) are matched after
their element's text. (b) every block's text is in the source at its
offsets, every row's `RowLocs` lie in its table and in file order (an
attribute's row may cite its element again), and each word of a row's
value cells is in the source at its `RowLocs`. All pass.

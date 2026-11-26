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
| parties (`SellerTradeParty`, `BuyerTradeParty`, `PayeeTradeParty`, `SellerTaxRepresentativeTradeParty`, `ShipToTradeParty`, `InvoiceeTradeParty` and the other `*TradeParty`) | a section with no heading; every block in it carries `inv.party` (`seller`, `buyer`, `payee`, `seller_tax_representative`, `ship_to`, ...). Each value is a paragraph with `inv.field` (the element's local name: `Name`, `ID`, `GlobalID`, `CityName`...) and `inv.group` for the aggregate it is in (`PostalTradeAddress`, `SpecifiedTaxRegistration`, `SpecifiedLegalOrganization`, `DefinedTradeContact`, ...). `schemeID` is `inv.schemeID` |
| references (`BuyerOrderReferencedDocument`, `AdditionalReferencedDocument`, ...), delivery event, billing period, payment means, payment terms | sections whose paragraphs carry `inv.group`; IBAN, BIC, mandate, due date as written, with `inv.field` (`IBANID`, `BICID`, `DirectDebitMandateID`, `DueDateDateTime` with `inv.format`); accounts carry `inv.account` |
| lines (`IncludedSupplyChainTradeLineItem`) | one table, a row per line with `RowLocs` |
| `ApplicableTradeTax` (header) | a one-row table each: tax type, VAT category, rate, basis, tax amount, exemption reason and code, other |
| `SpecifiedTradeAllowanceCharge` (header) | a one-row table each |
| `SpecifiedTradeSettlementHeaderMonetarySummation` | a one-row table |
| `AttachmentBinaryObject` | read generically for now (base64 text as an inferred paragraph, `mimeCode`/`filename` as attribute notes) |

**The lines table.** CII has no element around the lines, so
`SupplyChainTradeTransaction` is the table; the header agreement, delivery
and settlement it also holds are read as blocks (made before the table
block, which is made when the transaction ends). The table's header row is
the profile's (`header_source=profile`): line id, item name, description,
quantity (with `unitCode`), net price (`ChargeAmount`, then `BasisQuantity`
with `unitCode`), line net amount, VAT category, VAT rate, note, item
identifiers (seller's, buyer's, global with `schemeID`), gross price, item
attributes (`ApplicableProductCharacteristic`: description, value), billing
period, and **other**: everything else the line holds (origin country, tax
type, exemption reason, line allowances and charges, references, accounting
account), one value per line of the cell, in file order, each with its
`schemeID`/`listID`/`unitCode`/`currencyID`/`format`. Nothing of a line is
outside its row.

**Amounts** are as written, with `currencyID` after the value in the cell
text (`9.60 EUR`), through the profile key `with`; CII writes currency on
few amounts (`TaxTotalAmount`), the invoice currency being
`inv.currency`. In paragraphs the attribute is in `Attrs` (`inv.currencyID`).

CII has no element around the allowances or the VAT breakdown either, so
each is a table of one row (`one_row`), not one table for all.

## Corpus (dev, 223 files)

123 CII files read with `cii`; 82 UBL with `ubl`; 17 generic (16 ZUGFeRD
1.0, 1 `CBFBUY` invoice). One file declares `encoding="utf-8-w/o-bom"` and
is `ErrUnsupported` (the README's rule for unknown encoding labels). 19 CII
files hold an `AttachmentBinaryObject`.

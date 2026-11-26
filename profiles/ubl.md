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
| `cbc:Note`, `BuyerReference`, `AccountingCost`, `DueDate`, `TaxPointDate` | paragraphs with `inv.field` |
| parties (`AccountingSupplierParty`, `AccountingCustomerParty`, `PayeeParty`, `TaxRepresentativeParty`, `Delivery`, ...) | a section with no heading; blocks carry `inv.party` (`seller`, `buyer`, `payee`, `tax_representative`, `delivery`); values are paragraphs with `inv.field` (`EndpointID`, `Name`, `CompanyID`, `StreetName`, `Country/IdentificationCode`...), `inv.group` (`PartyName`, `PostalAddress`, `PartyTaxScheme`, `PartyLegalEntity`, `Contact`...), `inv.schemeID` |
| `PaymentMeans`, `PaymentTerms`, document references, `InvoicePeriod` | sections with `inv.group`; the IBAN is `PayeeFinancialAccount/ID` with `inv.account`, the BIC `FinancialInstitutionBranch/ID`; `PaymentMeansCode@name` is `inv.name` |
| lines (`cac:InvoiceLine`, `cac:CreditNoteLine`) | one table, a row per line with `RowLocs` |
| `cac:TaxTotal` | a table, a row per `TaxSubtotal` (`RowLocs`); the total `TaxAmount` is a paragraph (`inv.field=TaxTotal/TaxAmount`, `inv.currencyID`). A `TaxTotal` with no subtotal (the tax currency's) makes only that paragraph |
| `cac:AllowanceCharge` (document level) | a one-row table each |
| `cac:LegalMonetaryTotal` | a one-row table |
| `EmbeddedDocumentBinaryObject` | read generically for now; `cac:ExternalReference/cbc:URI` is a paragraph as written, never fetched |

**The lines table.** The root element is the table (UBL has no element
around the lines); everything else in the document is read as blocks made
before it. Columns (the profile's header row): line id, item name,
description, quantity (`unitCode`), net price (`PriceAmount` with
`currencyID`, then `BaseQuantity`), line net amount, VAT category, VAT
rate, note, item identifiers (seller's, buyer's, standard with
`schemeID`), item classification (`ItemClassificationCode` with `listID`
and `listVersionID`), item attributes (`AdditionalItemProperty`: name,
value), billing period, and **other**: everything else in the line
(accounting cost, order line reference, document references, line
allowances and charges, price allowance, origin country, tax scheme), one
value per line of the cell, in file order, with its attributes.

**Amounts** keep `currencyID` after the value in cells (`25.00 EUR`) and as
`inv.currencyID` in paragraphs.

## Corpus (dev)

82 files (75 `Invoice`, 7 `CreditNote`) read with `ubl`; 14 hold an
`EmbeddedDocumentBinaryObject`.

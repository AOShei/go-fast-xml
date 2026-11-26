# Profile `eforms`: EU eForms public procurement notices

The notices the Publications Office publishes on TED since 2023 (eForms
SDK 1.x): UBL 2.3 documents with the eForms extensions (`efext`, `efac`,
`efbc`).

## Selection

Root `ContractNotice`, `ContractAwardNotice` or `PriorInformationNotice`
in its UBL namespace
(`urn:oasis:names:specification:ubl:schema:xsd:ContractNotice-2`,
`...:ContractAwardNotice-2`, `...:PriorInformationNotice-2`). The same
names in another namespace or none stay generic, as does the SDK's
`BusinessRegistrationInformationNotice` (2 dev files, another schema);
`ubl` still takes invoices and credit notes (`TestEformsSelection`).

## Mapping

A notice is data: a few titles and descriptions, and several hundred
coded values per lot. The titles and descriptions are headings and
paragraphs; every other value is a row of a table of names and values
(`pairs`, see `PROFILE_FORMAT.md`), the name being the file's own
element path, attributes as rows of their own. Nothing is interpreted:
`selec-w`, `LOT-0001` and `32014L0024` are written as the file writes
them.

| Element | Output |
|---|---|
| root `cbc:ID`, `ContractFolderID`, `VersionID`, `IssueTime`, `RegulatoryDomain`, `NoticeTypeCode` (+ `listName`), `NoticeLanguageCode` (+ `listName`, `listID`), `UBLVersionID`, `CustomizationID`, `ProfileID`, `RequestedPublicationDate`, `PlannedDate` | `Metadata.Extra` under `eforms.` (`eforms.notice_id`, `eforms.notice_type`, ...) |
| root `cbc:IssueDate` | `Metadata.Created`; written with a time zone (`2025-01-14+00:00`) it is not an ISO date and stays as written in `Extra["created"]` |
| root `cac:ProcurementProject/cbc:Name` | `Metadata.Title` and a level-1 heading; a notice in several languages has one heading per `Name`, their `languageID`s in `Extra["eforms.title_language"]` |
| root `ProcurementProject/Description`, `Note` | paragraphs, `Attrs["eforms.field"]`, `eforms.languageID` |
| other children of the root `ProcurementProject` | a table each, names from `ProcurementProject/` (`ProcurementProject/MainCommodityClassification/ItemClassificationCode`, `.../@listName`) |
| `cac:ProcurementProjectLot` | a section: its `cbc:ID` (`LOT-0001`) is a level-2 heading and `Loc.Anchor`; every block of the lot carries `anchor_parent` |
| the lot's `TenderingTerms`, `TenderingProcess` | a table each |
| the lot's `ProcurementProject/Name` | a level-3 heading, `eforms.languageID`; `Description`, `Note` paragraphs; its other children a table each |
| root `ContractingParty`, `TenderingTerms`, `TenderingProcess`, `TenderResult`, `AdditionalNoticeLanguage` | a table each |
| `efext:EformsExtension` of the root: each `efac:Organization` (and `UltimateBeneficialOwner`) | a table |
| `efac:NoticeResult`: each `LotResult`, `LotTender`, `SettledContract`, `TenderingParty`, the total amounts | a table each, names from `NoticeResult/` |
| other children of the extension (`NoticeSubType`, `Publication`, `Changes`, `Appeals`, ...) | a table each |

Every row's `RowLocs` entry is the value's element. Extensions nested
inside a lot or a criterion are rows of the table they stand in.

**Field identifiers.** The eForms business terms (`BT-21`, `BT-137`) are
not in the files: they are in the SDK's `fields.json`, which maps each to
an XPath. The profile does not add them (a profile adds no words); the
row names are those XPaths' element names, so a consumer holding the SDK
can join on them.

**Blocks before the first heading** (organisations, results, the buyer)
belong to the notice as a whole: the file writes them before the title.

## Measured (dev: 524 files; held-out: 170)

The SDK's example notices and a sixth of one day of TED (15 January
2025); the dev files carry 24 different `languageID` values.

- dev: 522 read with `eforms` (2 `BusinessRegistrationInformationNotice`
  generic), 0 errors, 0 `Validate` failures; blocks: heading 4,160,
  paragraph 3,001, table 29,983, stored `markup_comment` 3,649;
  `xml.unmapped_elements` none, `xml.unmapped_attributes` none.
- held-out: 170 read with `eforms`, 0 errors, 0 `Validate` failures;
  heading 1,317, paragraph 929, table 9,416; nothing unmapped.

## Oracles (`TestEformsOracles`, 522 dev files)

(a) The body's text in order is the file's character data in order,
apart from the root values that go to `Metadata` (the list is in
`eformsCase`, each met on dev). (b) Every block's text is in the source
at its offsets, and every row's at its `RowLocs`. Both pass on every
file.

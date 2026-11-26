# Profile `pubmed`: PubMed / MEDLINE citations

Abstracts and bibliographic records of the NLM PubMedArticle DTD
(`pubmed_250101.dtd`): E-utilities `efetch` results and baseline files.

## What selects it

The root `PubmedArticleSet` in no namespace. The DOCTYPE
(`-//NLM//DTD PubMedArticle, ...`) is not tested: every dev file has it,
but the root name is the DTD's own and a file saved without its DOCTYPE
is the same data. A `PubmedArticleSet` in a namespace stays generic, as
do the DTD's other roots (`PubmedArticle` alone, `PubmedBookArticleSet`,
`BookDocumentSet`) and the older `MedlineCitationSet`: none is in the
corpus to test against (`TestPubmedSelection`).

## Records

| Source | Output |
|---|---|
| `PubmedArticle`, `PubmedBookArticle` | A `section` per record; its anchor is the PMID as written (`MedlineCitation/PMID`, `BookDocument/PMID`), and the PMID is also the record's **heading, level 1**, where it stands (`"heading": "anchor"`), with the PMID as `Loc.Anchor`. The file puts a record's dates and journal before its title, so without it those tables would stand before any heading of their record. Every other block of the record carries the PMID in `anchor_parent`; `PMID@Version` is `pubmed.pmid_version` on the record's first block |
| `MedlineCitation/Article/ArticleTitle` | Heading level 2 |
| `Book/BookTitle`, `BookDocument/ArticleTitle` | Headings 2 and 3, in file order; `@book`, `@part`, `@sec` as `pubmed.book`, `pubmed.part`, `pubmed.sec` |
| `VernacularTitle` | A paragraph with `pubmed.field=VernacularTitle`: a title in the original language, read as its own block where the file puts it (after the publication types), not a heading, so as not to split the record |
| `MedlineCitation@Status`, `@Owner`, `@IndexingMethod`, `@VersionID`, `@VersionDate`; `Article@PubModel` | `pubmed.status`, `pubmed.owner`, `pubmed.indexing_method`, `pubmed.version_id`, `pubmed.version_date`, `pubmed.pub_model` on the first block made inside the element (a dates or journal table) |
| `Abstract`, `OtherAbstract` | Their blocks are marked `pubmed.in`; `OtherAbstract@Language` and `@Type` as `pubmed.language`, `pubmed.abstract_type` |
| `AbstractText` | A paragraph; `@Label` is its `Marker` (written in front of it by `Text()` and `Markdown()`), `@NlmCategory` is `pubmed.nlm_category` |
| `CopyrightInformation`, `CoiStatement`, `GeneralNote` | Paragraphs with `pubmed.field`; `GeneralNote@Owner` as `pubmed.owner` |
| `AuthorList`, `InvestigatorList` | A table, header the profile's: last name, fore name, initials, suffix, collective name, identifier (`Identifier` with its `Source` after it), affiliation (each `AffiliationInfo` a line: `Affiliation`, then its `Identifier`s with their `Source`). A row per author, `RowLocs` on the `Author` element. `@CompleteYN`, `@Type` as `pubmed.complete`, `pubmed.list_type`. The rows' attributes (`ValidYN`, `EqualContrib`) are a rest table right after it: last name, `Author/@ValidYN`, value |
| `KeywordList` | A list, an item per `Keyword` (inline markup kept), `@MajorTopicYN` as `pubmed.major_topic`, the list's `@Owner` as `pubmed.owner` |
| `ReferenceList` | A list (`pubmed.in=ReferenceList`), an item per `Reference` with its `Citation` text; its `ArticleIdList` is a name and value table after the item (`ArticleId`, `ArticleId/@IdType`). `ReferenceList/Title` is a heading level 3 |
| `Sections/Section` (books) | A list, an item per `Section`: `LocationLabel` and `SectionTitle`; a nested `Section` is an item one level deeper. `SectionTitle@book/@part/@sec` and `LocationLabel@Type` in `Attrs` |
| `ItemList/Item` (books) | A list, `@ListType` as `pubmed.list_type` |
| `DeleteCitation` | A name and value table (`PMID`, `PMID/@Version`), `pubmed.field=DeleteCitation` |
| `i`, `b`, `sup`, `sub`, `u`, MathML | Inline, as in `jats` (`DispFormula` a formula block) |

Every other group is a **name and value table** (`pairs`, header
`name | value`, the names the file's paths starting with the group's own
element (`Journal/ISSN`), attributes as `<name>/@<attribute>`
rows, `RowLocs` on each value's element), as the invoice profiles read
parties: `DateCompleted`, `DateRevised`, `ArticleDate`, `ContributionDate`,
`Journal` (ISSN, `JournalIssue` with volume, issue, `PubDate`, title,
abbreviation), `Pagination`, `ELocationID`, `Language`, `DataBankList`,
`GrantList`, `PublicationTypeList`, `MedlineJournalInfo`, `ChemicalList`,
`SupplMeshList`, `CitationSubset`, `CommentsCorrectionsList`,
`GeneSymbolList`, `MeshHeadingList` (`MeshHeadingList/MeshHeading/DescriptorName`,
`.../@UI`, `.../@MajorTopicYN`, `.../QualifierName`, ...),
`NumberOfReferences`, `PersonalNameSubjectList`, `OtherID`,
`SpaceFlightMission`, `History`, `PublicationStatus`, `ArticleIdList`,
`ObjectList`, and in books `Publisher`, `PubDate`, `BeginningDate`,
`EndingDate`, `Volume`, `Edition`, `Medium`, `ReportNumber`, `Isbn`,
`LocationLabel`. A date written as `Year`/`Month`/`Day` (and `Hour`,
`Minute`) is kept as its named parts (`PubMedPubDate/Year`, ...), with
`PubMedPubDate/@PubStatus` before them; `MedlineDate` as written. No code
is interpreted.

`Metadata` is not filled from a record, whether the file holds one record
or several: the profile cannot tell them apart, and a set's first record
is not the file's title. No `Extra` key is written.

## Left to generic reading

Nothing on dev: `xml.unmapped_elements` and `xml.unmapped_attributes`
are empty. Rules for elements of the DTD that dev does not have
(`DispFormula`, MathML, `ObjectList`, `GeneSymbolList`, `Isbn`, ...) are
written from the DTD and the fixtures.

Known limit: a value in a name and value table that starts with inline
markup (`<PublisherName><i>X</i> Press</PublisherName>`) is read by the
engine as values of the inline elements (named `i`); keywords are a list
for that reason. Not met on dev in the other tables.

## Dev set (146 files)

146 read, 0 errors, 0 `Validate` failures, unmapped elements and
attributes none. Blocks: heading 2897, list_item 25075, paragraph 3244,
table 39921. Warnings: `input.external_reference` 146 (the external DTD,
not loaded).

Oracles (`TestPubmedOracles`): (a) the body's block and cell text in
order equals the file's character data in order, white space aside, with
no exception; the profile's
header rows, the name column and attribute rows of name and value tables,
an identifier's `Source` and `AbstractText@Label` (the marker) are not
compared, being attribute or profile text. (b) Every block's text is in
the source at its offsets. Both pass.

## Held-out set (53 files)

53 read, 0 errors, 0 `Validate` failures, unmapped elements and
attributes 0. Blocks: heading 1192, list_item 10909, paragraph 1366, table
15893. Warnings: `input.external_reference` 53, `part.no_text` 1.

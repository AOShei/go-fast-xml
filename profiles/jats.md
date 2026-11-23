# Profile `jats`: JATS and NLM journal articles

## What selects it

The root element `article` (no namespace) with a DOCTYPE public
identifier starting `-//NLM//DTD` or `-//NISO//DTD`, or with an
`article-type` or `dtd-version` attribute; or the root `pmc-articleset`
(PMC's efetch wrapper, each `article` read in turn). JATS has no
namespace; `xlink:`, `mml:` (MathML), `ali:` and `oasis:` (CALS tables)
are resolved by URI.

## Parts

| Element | Part |
|---|---|
| `article/front` | `body` part named `front` |
| `article/body` | `body` part named `body` |
| `article/back` | `body` part named `back` |
| `article/floats-group` | `body` part named `floats-group` |
| `sub-article`, `response` | `chapter` part named by its `article-type` / `response-type`; every block carries `Attrs["jats.sub_article"]` = its `id` |

The part names are the element names (the file has no other name for
them); a reader that wants one body can ignore the parts.

## Mapping

| Element | Model |
|---|---|
| `article-meta/title-group/article-title` | `Metadata.Title` and a level-1 heading in the front part |
| `article-meta/contrib-group/contrib/name` (also `collab`, `string-name`, `name-alternatives`) | `Metadata.Authors`, one per contributor, `Surname Given` as written |
| `contrib` | paragraph with the rest of the contributor (ORCID `contrib-id`, `role`, `xref` to affiliations, `email`, `degrees`), anchor `id`, `Attrs` `jats.field=contrib`, `jats.contrib_type`, `jats.corresp`, `jats.equal_contrib`, `jats.deceased` |
| `aff` | paragraph, `Marker` from `label`, anchor `id`, `jats.field=aff` |
| `author-notes` | its `corresp` paragraphs and `fn` footnotes, marked `jats.in=author-notes` |
| `kwd-group/kwd`, `compound-kwd` | `Metadata.Keywords` |
| other `article-meta` and all of `journal-meta` | `Metadata.Extra` (below) |
| `abstract`, `trans-abstract` | section (its `title` a heading, if it has one; none is invented), blocks marked `jats.in=abstract` |
| `sec` | section; `label` is the heading's `Marker`, `title` the heading, anchor `id` on the heading |
| `ack`, `app-group`, `app`, `notes`, `glossary`, `fn-group`, `ref-list` | sections, `title` a heading with the element's `id` as anchor |
| `p` | paragraph, anchor `id` |
| `list` | list; ordered when `list-type` is `order`, `alpha-*` or `roman-*`; `label` of an item is its `Marker` |
| `def-list`/`def-item` | list; the `term` is the item, the `def` paragraphs follow at the item's depth |
| `disp-quote`, `verse-group` | quote |
| `preformat`, `code` | code (`Attrs["language"]` from `code@language`) |
| `boxed-text`, `statement` | their blocks marked `jats.box`; `caption` a caption block |
| `fig`, `fig-group` | `label` the `Marker` of the `caption` block (anchor = the fig's `id`); `graphic` an image block (id = `xlink:href`, alt from `alt-text`) |
| `table-wrap` | `label` a caption block, `caption` a caption block (both anchored at the table-wrap's `id`), `table` (XHTML, `thead/tr` header rows, `colspan`/`rowspan`) a table; `oasis:table` read as CALS; `table-wrap-foot` notes as footnotes |
| `disp-formula` | MathML: a `formula` block, notation `plain`, text content, `Attrs["jats.math"]=mathml` (the part carries `input.unread_content` naming MathML); `tex-math`: `formula`, notation `latex`; `label` is the `Marker`; anchor = the formula's `id`; `alternatives` keeps every alternative |
| `inline-formula`, `mml:*` in text | inline: the MathML's text stays in its paragraph |
| `fn` | footnote, anchor `id`, `Marker` from `label` |
| `ref` | footnote, anchor `id` (what `xref@rid` points at), `Marker` from `label`, `jats.field=ref`; the citation's fields (`element-citation`, `mixed-citation` and their children) are its text, set off by spaces (`Smith J 2010 On tests Nature 1 2 10.1/x`) |
| `supplementary-material`, `media` | caption block; `xlink:href` in `Attrs["jats.href"]` of the first block |
| `xref` | link span to the target's `id` (`rid`) |
| `ext-link`, `uri`, `pub-id`, `related-article` (in text) | link span to `xlink:href` |
| `inline-graphic` | a record in the stored part (`custom_property`, `href` the file) |
| `italic`, `bold`, `sup`, `sub`, `monospace` | spans; `underline`, `sc`, `named-content`, `styled-content`, `abbrev` and the like stay in the text with no style |
| `object-id` | paragraph, `jats.field=object-id` (the DOI of a figure or table) |
| `permissions` outside `article-meta` (figures, sub-articles) | paragraphs, `jats.field` the element name, marked `jats.in=permissions` |
| `front-stub` of a sub-article | content of its part: `article-title` a level-1 heading, contributors as paragraphs with their names, ids, dates and permissions as paragraphs (`jats.field`) |

A `p` may hold a `fig`, `table-wrap`, `disp-formula`, list or `fn`.
Their text is then joined into the paragraph (line breaks between), their
labels kept at their place in it; the `graphic` is an image block before
the paragraph. The format has no rule that ends a paragraph at a nested
block.

## Metadata.Extra keys

`jats.article_type`, `jats.dtd_version`, `jats.lang`, `jats.specific_use`
(attributes of `article`); `jats.journal_id` (+ `jats.journal_id_type`,
parallel, `; `-separated), `jats.journal`, `jats.journal_subtitle`,
`jats.journal_abbrev`, `jats.issn` (+ `_type`, `_format`), `jats.isbn`,
`jats.publisher`, `jats.publisher_loc`; `jats.doi`, `jats.pmid`,
`jats.pmcid`, `jats.pmc`, `jats.publisher_id`, `jats.article_id` (+
`_type`, other id types), `jats.article_version`; `jats.subject` (+
`jats.subject_group_type`), `jats.series_title`; `jats.alt_title`,
`jats.trans_title`, `jats.trans_subtitle`; `jats.pub_date` (as written,
`day month year` in the file's order, + `_type`, `_date_type`, `_format`,
`_iso` from `iso-8601-date`), `jats.volume`, `jats.issue`,
`jats.issue_id`, `jats.issue_title`, `jats.fpage`, `jats.lpage`,
`jats.page_range`, `jats.elocation_id`; `jats.history` (+ `_type`,
`_iso`), `jats.pub_history`; `jats.copyright`, `jats.copyright_year`,
`jats.copyright_holder`, `jats.license` (+ `_href`, `_type`),
`jats.license_ref`, `jats.free_to_read`; `jats.self_uri` (+ `_href`,
`_type`); `jats.kwd_group_type`, `jats.kwd_group_title`; `jats.funding`
(each `award-group` as text) and `jats.funding_statement`;
`jats.custom_meta` (`name value`); `jats.page_count`, `jats.fig_count`,
`jats.table_count`, `jats.equation_count`, `jats.ref_count`,
`jats.word_count`; `jats.tagset_family`, `jats.base_tagset`,
`jats.table_model`, `jats.mathml_version`, `jats.restricted_by`.
Dates are kept as written: the profile does not build ISO dates from
`day`/`month`/`year`; `iso-8601-date` is copied where the file gives it.

## Attrs keys

`jats.field` (what a paragraph made from a front-matter or other
non-paragraph element is: `contrib`, `aff`, `corresp`, `subtitle`,
`object-id`, `label`, `ref`, ...), `jats.in` (`abstract`,
`trans-abstract`, `ack`, `app`, `author-notes`, `permissions`,
`table-wrap-foot`), `jats.box` (`boxed-text`, `statement`,
`supplementary-material`), `jats.sec_type`, `jats.abstract_type`,
`jats.list_type`, `jats.fn_type`, `jats.fig_type`, `jats.content_type`,
`jats.position`, `jats.math`, `jats.href`, `jats.mimetype`,
`jats.notes_type`, `jats.sub_article`, `jats.lang`.

## How to cite a block

`Loc.Offset`/`Length` give the element in the file. `Loc.Anchor` is the
JATS `id` of the block's element (`p`, `fn`, `ref`, `aff`, `contrib`,
`disp-formula`) or, for a heading, of its `sec`; a figure's or table's
caption carries the `fig`'s or `table-wrap`'s `id`. Other blocks carry
the nearest enclosing `id` in `Attrs["anchor_parent"]`. With `jats.doi`
(or `jats.pmcid`) this gives a citation such as `10.7554/eLife.00013#fig1`.
A `contrib` paragraph's text is in its span with the name left out.

## Left to generic reading

None of the elements met in the dev set. Attributes not read (counted in
`xml.unmapped_attributes`, values not in the output): presentation
(`td@align`, `@valign`, `@style`, `col@width`, `@orientation`,
`@position` outside fig/table-wrap, MathML presentation attributes),
`name@name-style`, `italic@toggle`, `xref@ref-type`, `pub-id@pub-id-type`,
`ext-link@ext-link-type`, `element-citation@publication-type`,
`person-group@person-group-type`, `named-content@content-type`: core's
`Span` has no field for them and a stored record per citation would
outweigh the text.

## Versions

Samples: JATS 1.1 and 1.1d3 Archiving (eLife), NLM Journal Publishing
3.0 (PLOS), PMC article sets 2.0 (JATS 1.x and 1.3 content, including
`processing-meta`). Not seen in the samples: JATS 1.2/1.3/1.4 Authoring
(BITS-like `book` roots are not matched), `oasis:table` CALS tables,
`response`, `trans-abstract`, `verse-group`, `code`.

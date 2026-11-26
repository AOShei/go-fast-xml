# Writing a profile

A profile tells go-fast-xml what the elements of one XML schema mean: which
element is a heading, a paragraph, a list, a table, a link, a piece of
metadata. Without one, a file is read generically (see the README). This
is the format of profile version 1, as signed off on 2026-11-23 (part A
of the design document), written as instructions. The few keys added
while building the Service-Public profile are marked **(added)** and
listed at the end.

A profile is one JSON file. It is data: nothing in it is run, and it
cannot reach outside the document. **Unknown keys are an error** when a
profile is loaded, so that a typo cannot silently change the output.

```json
{
  "profile": "service-public",
  "title": "Service-Public.fr, Publication schema 3.5 (DILA)",
  "format": 1,
  "namespaces": {"dc": "http://purl.org/dc/elements/1.1/"},
  "match": [
    {"root": "Publication"},
    {"root": "ServiceComplementaire"}
  ],
  "heading_base": 2,
  "rules": [
    {"match": "dc:title", "as": "metadata", "field": "title", "also_heading": 1},
    {"match": "Chapitre", "as": "section"},
    {"match": "Chapitre/Titre", "as": "heading", "level": "nesting"},
    {"match": "Paragraphe", "as": "block", "kind": "paragraph"},
    {"match": "Liste", "as": "list", "ordered": {"attr": "type", "in": ["numero"]}},
    {"match": "Liste/Item", "as": "item"},
    {"match": "MiseEnEvidence", "as": "inline", "style": "bold"},
    {"match": "LienExterne", "as": "inline", "link": {"attr": "URL"}}
  ]
}
```

| Key | Meaning |
|---|---|
| `profile` | The name: lower-case letters, digits, `.`, `_`, `-`; not `none`. It is what `Metadata.Extra["xml.profile"]` records |
| `title` | A description, shown by `gofastxml -profiles` |
| `format` | Always `1` |
| `namespaces` | Prefixes used in selectors, mapped to namespace URIs |
| `default_namespace` | The namespace of unprefixed element names; absent means no namespace |
| `match` | When the profile applies (below) |
| `heading_base` | The level of the outermost section's heading (`"nesting"`), 1 to 6; default 1 |
| `rules` | The rules |
| `notes` **(added)** | Anything, for people (a draft's account of its schema); not read |

A rule may carry `"note"` (a string, for people; not read). A rule with
`"review": true` is refused: it is a suggestion in a draft written by
`gofastxml -draft-profile` (README, "Drafting a profile from an XSD"),
and a draft is not a profile until each rule is reviewed and its mark
removed. `"review": false` is accepted and means nothing.

Load one with `gofastxml.LoadProfile`; a profile that does not load gives
an error naming the rule (`profile: rule 12 ("Chapitre/Titre"): ...`).

## 1. Choosing a profile

`match` is a list; a file matches when any entry does. An entry tests the
root element: `root` (local name), `namespace` (URI; absent means "no
namespace"), and optionally `doctype_public` (a prefix of the DOCTYPE
public identifier), `attr` (attribute tests on the root, written as in
selectors: `"[@class~='topic/topic']"`) and `declares` **(added)** (a
namespace URI the root element declares, with any prefix or as the
default: RSS 1.0's `rdf:RDF` declares `http://purl.org/rss/1.0/`).

The profiles in `Extractor.Profiles` are tried first, in order, then the
built-in ones in a fixed order, then generic reading. `Extractor.Profile`
(`-profile NAME|FILE` on the command) forces one whatever the root;
`none` forces generic reading. `Metadata.Extra["xml.profile"]` records the
profile used.

## 2. Selectors

`"match"` in a rule is a selector, matched against the end of the
element's path:

```
selector  = ["/"] step {"/" step}          leading "/" anchors at the root
step      = name | "*"                      name is local or prefix:local, prefixes from "namespaces"
            { "[@a]" | "[@a='v']" | "[@a~='token']" }
```

`Chapitre/Titre` is a `Titre` whose parent is `Chapitre`. `[@a~='t']`
tests one space-separated token (DITA's `class`). There are no other
axes, no positions and no functions. When several rules match, the one
with more steps wins, then the one with more attribute tests, then the
later rule. An unprefixed element name means "no namespace" unless the
profile sets `"default_namespace"`; an unprefixed attribute name is always
in no namespace. The prefix `xml` is predefined.

## 3. Rule kinds (`"as"`)

| `as` | Meaning | Keys |
|---|---|---|
| `section` | A container that deepens heading nesting | |
| `heading` | A heading block | `level`: a number, `"nesting"` (`heading_base` + number of enclosing `section`s - 1), or `{"attr": "level"}`; `marker`; `duplicate_of` and `key` **(added)** |
| `block` | One block of `kind` `paragraph`, `quote`, `code`, `caption`, `footnote`, `formula` | `kind`; for `code`, `language`; for `formula`, `notation` (`latex`, `unicodemath`, `plain`); `marker`; `value` **(added)**; `duplicate_of` and `key` **(added)** |
| `list`, `item` | A list and its items; depth from nesting | `ordered`: `true`, `false` or an attribute test `{"attr": "type", "in": ["numero"]}`; on `item`: `marker` |
| `table` | A table | `model`: `rows` (default) or `cals`; `header`, `one_row`, `rest`, `pairs`, `rest_table` **(added)** |
| `row`, `cell` | In the `rows` model | `row`: `header` (`true` or an attribute test), `cells` **(added)**, `rest`, `rest_table` **(added)**; `cell`: `header`, `colspan`, `rowspan` (attribute names), `column` and `with` **(added)** |
| `inline` | Text that stays inside its block | `style`: `bold`, `italic`, `superscript`, `subscript`, `code`; `link`: a value source for the address; `title` and `attrs` **(added)**; `space` **(added)**; `value` **(added)** |
| `break` **(added)** | A line break written as an element (`br`, `lb`, `eol`): a `\n` in the block it stands in | |
| `label` | Its text becomes the `Marker` of the block it belongs to (the parent block, or the next block in the same container) | |
| `image` | An image block | `id` (value source for the address or identifier), `alt` (value source) |
| `note` | A `note` block | `type`: one of core's note types; `value` **(added)** |
| `metadata` | A metadata value; makes no block | `field`: `title`, `author`, `created`, `modified`, `language`, `keyword`, `subject`, `description`, or `extra` with `key`; `value` (value source, default the element's text); `also_heading`: a level, to also emit the title as a heading where it stands; `join` **(added)** |
| `part` | Starts a new part | `kind`: `body` or `chapter`; `name` (value source) |
| `stored` | The element's text goes to the stored part | `kind`: one of core's stored kinds |
| `transparent` | The element means nothing itself; its children are read by their own rules | |
| `skip` | Left out, counted in the info warning `xml.skipped_elements`. Refused in built-in profiles | |
| `document` **(added)** | The element's content is an embedded document, handed to `Options.ExtractChild` and appended to `Document.Children` (below) | `media_type`; `when` |

A `document` rule's `media_type` is `"text/html"` or
`"application/xhtml+xml"`, or chosen by an attribute:
`{"attr": "type", "map": {"html": "text/html", "xhtml": "application/xhtml+xml"}}`
(1 to 64 entries, values exact). When the attribute is absent or its
value is not in the map, the rule does not apply and the element's other
rule (or generic reading) does; a rule with a map counts as having one
more attribute test when rules are ranked. `"when": "markup"` makes the
rule apply only when the element's character data, unescaped, holds a tag
(`<` followed by a letter, `/` or `!`, and later a `>`) or a reference
(`&name;`, `&#digits;`, `&#xhex;`), or the element has child elements.

What is handed over: for `text/html`, the character data unescaped
(entities and CDATA resolved), with the tags of any child elements
written back, as UTF-8, media type `text/html; charset=utf-8`; for
`application/xhtml+xml`, the bytes of the file between the start and end
tag when the file is UTF-8, else the content written back from the
decoded text, media type `application/xhtml+xml; charset=utf-8`.
Namespace declarations in scope but outside the element are not added.
The child's name is the element and its parent and the anchor of the
enclosing identified element (`item/description#guid`, the anchor cut to
256 bytes). At most 1 MiB (`MaxBlockBytes`) is handed over; a longer
content is cut there, with `limit.exceeded`.

The rule's **fallback** is a `block` of kind `paragraph` with the rule's
`anchor`, `attrs` and `mark`: its text is the character data with the
markup as text (HTML), or the text content (XHTML; a space between
block-level elements). The fallback is made when `when` finds no markup
(no `media_type` attribute), when the child was not extracted (no
`Options.Child`, the depth limit, the child extractor refused or failed:
then with `Attrs["media_type"]`), and past `MaxChildren` (10,000)
documents (with `media_type` and `limit.exceeded`). When the child was
extracted the body has no block for the element. Met inside an open
block, a `document` rule's text stays in that block.

The two links: the heading of the innermost `section` the element stands
in (else the section's first block) carries `Attrs["children"]`, the
indexes in `Document.Children` of the documents from that section, comma
separated; a section with no block has none. Each child carries in
`Metadata.Extra` `xml.parent_offset` and `xml.parent_length` (its element
in the file, start tag to end tag), `xml.parent_anchor` (the enclosing
identified element's anchor, when there is one, also when it comes after
the element) and `xml.parent_element` (the element's path). A child that
was not extracted (the stub `ExtractChild` returns) is listed with the
same keys.

Keys any block-making or container rule may carry:

- `anchor`: value source for `Loc.Anchor` (`{"attr": "ID"}`). `xml:id` is
  used when there is none.
- `attrs`: `{"key": value source}` copied to the block's `Attrs` (keys are
  the profile's own, written as given; core's `stored`, `type` and `role`
  are refused). On a `section`, `transparent`, `list` or `part` rule, which
  makes no block of its own, they go to the first block made inside it
  (a section's heading).
- `mark`: like `attrs`, but on a container: every block made inside it
  carries the key. This is how conditional content and labelled boxes are
  expressed (DITA `audience`, DocBook `condition`, S1000D applicability,
  Service-Public `Condition`).
- `display`: `{"attr": "…", "in": ["…"], "reason": "hidden"}` (or
  `"no_ink"`) where the schema has a switch that declares content not
  shown.
- `extra` **(added)**: `{"key": value source}` copied to
  `Metadata.Extra`, for values of an element that has a rule of another
  kind (the root's attributes).

A profile cannot add words to the text. Text comes only from the file,
with one exception: a table's `header` row, which is marked as the
profile's (section 7). Text from the file includes attribute values a
rule names: a block's or note's `value`, and an inline's `value`, which
is written in place in the block's text (section 7).

## 4. Value sources

Where a rule takes a value:

| Form | Value |
|---|---|
| `"text"` | The literal string |
| `{"attr": "name"}` | The attribute of this element |
| `{"attr": "name", "of": "Ancestor"}` | The attribute of the nearest ancestor of that name |
| `{"child": "selector"}` | The text of the first matching child, which is then not emitted again. The selector is relative: `Theme/Titre` is a `Titre` in a `Theme` child |
| `{"attr": "name", "child": "selector"}` **(added)** | The attribute of the first matching child (`templateId@root`, `patient/administrativeGenderCode@code`). The child is still read by its own rule (or generically); the attribute is then not counted as unread nor made a note |
| `{"attr": "name", "codepoint": "hex"}` **(added)** | The character whose code point the attribute of this element holds in hexadecimal (Formex `QUOT.START@CODE="2018"` gives U+2018). A value that is not 1 to 6 hex digits, a surrogate, past U+10FFFF or a control (category Cc) gives nothing, and the attribute is counted in `xml.unmapped_attributes` |
| `{"anchor_of": "Element"}` **(added)** | In `anchor` only: the anchor of the nearest enclosing element of that name, as its rule's `anchor` gave it; nothing when it has none yet (section 7) |
| `{"structure": "selector"}` | A matching child written as an outline of its element names and attributes, e.g. `ou(estVrai(var=T1), estIndefini(var=T2))`, for machine-readable subtrees with no text; the child is then not emitted again |

A value read from a child is known when the child ends; the engine gives
it to blocks already made (for `mark`, every block made inside the
element so far) as well as those to come.

## 5. What the engine does without being told

- Text directly inside a container with a rule but no block (stray text
  in a `section`) becomes an inferred paragraph. No text is lost.
- An element with no rule is read generically: its text becomes an
  inferred paragraph (or stays in the block it stands in), its attributes
  notes, and it is counted in `xml.unmapped_elements` (info, names with
  counts). Attributes of **mapped** elements are not turned into notes:
  the profile says which matter (`attrs`, `mark`, `anchor`, value
  sources). Attributes of a mapped element that no rule reads are counted
  by name in `xml.unmapped_attributes` (info); their values are not in the
  output.
- A `block` inside a `cell` or `item` does not become a block of its own:
  paragraphs in a cell are joined with `\n`, a list in a cell is written
  one item per line with its marker; the first paragraph of an `item` is
  the item's text and later blocks follow as blocks at the item's depth.
  Blocks inside another `block` are joined with `\n` the same way.
- A `block` inside a `heading` or `label` is unwrapped (`Titre/Paragraphe`).
- A `part` met inside an open block (a paragraph, heading, item) finishes
  that block first: its text so far is a block in the part being left,
  its `Loc` ending where the part starts. Text after the part, still in
  the block's element, starts a new block (not `Continued`: it is in
  another part) whose `Loc` begins at that text. Inside a cell or a value
  read for a rule, a part is read into it as before.
- A `table` or `image` met inside an open paragraph-like `block` (a
  patent's `p/tables/table`, `p/chemistry/img`), or the element of a
  rule with `"split": true` (JATS `p/fig`, DocBook `para/footnote`, TEI
  `p/note`), finishes that block the same way and is read on its own, in
  reading order; text after it starts a new block. Inside a cell, an
  item, a heading or a value read for a rule it is read into it as
  before. A label waiting as the marker of a block that has no text when
  it is finished this way is that block's text.
- So a block met inside a paragraph is read one of three ways, chosen by
  its rule: by default it is a line of the paragraph; with `split` it
  ends the paragraph there (content stays in file order); with
  `out_of_line` (notes only: a footnote written inside its sentence) it
  is a block made just before the paragraph, which goes on unbroken.
- An `xi:include` (XInclude 2001 or its 2003 draft) is never followed:
  with or without a rule it raises core's `input.external_reference`
  (warning, once per document with the count), as in generic reading
  and as external DTDs and stylesheets are reported. Its `href` stays as
  text: the block of its rule (DocBook reads it as a paragraph whose
  text is the `href`), or an attribute note when it has no rule.
- Inline styles and links become `Spans` when `Options.Spans` is set (in a
  block split into `Continued` pieces, each span is on the piece that
  holds it, and a span across the cut is in both, with each piece's
  offsets); in a
  table, a cell's link goes to `Table.Cells` (also with `Options.Spans`,
  as core defines).
- An item of an ordered list with no `marker` has its number (`1.`) as
  `Marker`.
- Blocks made by a rule are `native`. Everything from generic fallback is
  `inferred`.
- A heading deeper than 6 has level 6 and its level in `Attrs["level"]`
  (`structure.heading_level_clamped`).
- Loading validates the profile: an unknown key or `as`, a `kind` or note
  type core does not have, an unresolvable prefix, a `skip` in a built-in
  profile are errors naming the rule.

## 6. Citing the source

Every block, whether made by a rule or by generic fallback, can be traced
to its place in the file:

- `Loc.Offset` and `Loc.Length` cover the block's element in the source
  (from the start of its start tag to the end of its end tag), in bytes of
  the file, as go-fast-html defines offsets. A block split into
  `Continued` pieces has the offsets of its own piece. A table's `Loc`
  covers the table element; a heading's covers its title element; a block
  made from an attribute covers the element that holds it; an item covers
  the item element; stray text covers the text.
- A table's rows are cited one by one: `Table.RowLocs` has one location
  per row, in the order of `Rows`: the row element's bytes, and its
  `Anchor` when the row rule has an `anchor`. A header row the profile
  supplies (`header`) has an empty location. Both table models and rows
  made from attributes (`cells`) give them.
- `Loc.Anchor` is the identifier the source gives the block: the rule's
  `anchor`, else `xml:id`. A block with no identifier of its own carries
  the nearest enclosing identified element's in `Attrs["anchor_parent"]`,
  when a rule names that element's identifier with `anchor`.
- Text moved out of the body (metadata values, `mark` and `attrs` values,
  the stored part) is not cited by offset; block text always is. A block
  whose text is an attribute's value carries the attribute's name in
  `Attrs["attribute"]`.

## 7. Keys added while building the Service-Public profile

Each is the smallest key that could say what the signed-off mapping asks
for; none changes what an existing key means.

| Key | On | Why |
|---|---|---|
| `duplicate_of`, `key` | `heading`, `block` | S4: `Cas` has `Titre` and `TitreRiche` with the same text. `{"match": "Cas/Titre", ..., "duplicate_of": "TitreRiche", "key": "sp.title_plain"}`: when the element and its adjacent sibling `TitreRiche` (before or after it) have the same text, the element gives nothing; when they differ, its text goes to `Attrs[key]` of the sibling's block. With no such sibling the element is read by its own rule |
| `title` | `inline` with `link` | S6: the title of a link's target (`commentaireLien`). Core's `Span` has no field for it, so a link with a `title` or `attrs` gives a record in the stored part: a `note` (`stored`=`custom_property`, `name` the element's name) whose text is the link's own text (its address when it has none), and whose `Attrs` hold `href`, the title under `title`, and the rule's `attrs`. The Service-Public profile puts `commentaireLien` in `attrs` as `sp.comment` |
| `attrs` | `inline` | The other attributes of a link (`type`, `audience`), in that record. On an inline that is not a link (`TermeEtranger@langue`) the record is made too, its text repeating the element's: core's `Span` has no language field to hold it (core v0.1.4) |
| `extra` | any rule but `label`, `skip` | Root attributes (`ID`, `spUrl`) into `Metadata.Extra` (C1) while the root stays `transparent` |
| `join` | `metadata` | The separator of repeated values of one `Extra` key (` > ` for the breadcrumb, C1); default a line break |
| `value` | `block`, `note` | A block whose text is an attribute (`RessourceWeb@URL`, C4); it carries `Attrs["attribute"]` |
| `cells` | `row` | A row whose data is in its element's attributes (`Redirection@from`, `@to`, ...): `"cells": [{"attr": "from"}, {"attr": "to"}]`, one cell per entry in order, 1 to 64 entries. Each entry is `{"attr": "name"}` of the row's own element (no literal, `of`, `child` or `structure`: a cell holds only what the element holds). An absent attribute gives an empty cell. The table carries the names, space-separated in order of first use, in `Attrs["attribute"]` |
| `space` | `inline` | Phase 3 (JATS, DocBook, TEI): `"space": true` sets the element's text off by white space: a space is held before and after it (collapsed with white space the source has, dropped at a block's ends), also in metadata values and values read from a child. For fields the schema writes with no white space between them (`<surname>Krusche</surname><given-names>Adele</given-names>` gives `Krusche Adele`, not `KruscheAdele`). It adds no words; the text stays the file's |
| `header` | `table` | A header row of fixed text: `"header": ["from", "to"]` (1 to 64 strings of at most 256 bytes). It is the profile's text, not the file's, so the table carries `Attrs["header_source"]="profile"` (core prescribes nothing for text a file does not hold; this is the library's choice). The table stays `Origin: native`: core's `inferred` is structure deduced by guessing, and here every row and cell is declared by the file. It is the table's first row and counts in `HeaderRows`. On a `row` or `cell`, `header` keeps its meaning (a test) |
| `namespace_aliases` | profile | Phase 3 (Akoma Ntoso): one schema published under several namespaces (AKN 3.0, its draft `3.0/CSD03`, `akomantoso.org/2.0`). `"namespace_aliases": {"http://www.akomantoso.org/2.0": "http://docs.oasis-open.org/legaldocml/ns/akn/3.0"}`: an element in an alias namespace is matched by the rules as if it were in the target namespace. Attributes are not affected; the target may not itself be an alias; at most 64. The `match` entries still name each namespace the profile is chosen for |
| `out_of_line` | `block`, `note` | Phase 3 (Akoma Ntoso `authorialNote`, Formex `NOTE`, CLML `Footnote`): a note written inside the sentence it annotates. `"out_of_line": true`: met inside an open paragraph, heading or item, the element is a block of its own instead of a line of that block, made where it ends, so it comes just before the block it stands in (made at its end); that block goes on after it with the rest of its text and keeps a waiting `label`. Its `Loc` covers its element, inside the span of the block it stands in, whose text is the span's minus the note. In a cell, or a value read for a rule, it stays inline as before |
| `split` | `block`, `note`, `list`, `transparent` | Phase 3 integration (JATS `fig`, `table-wrap`, `fn`; DocBook `footnote`, `programlisting`, admonitions; TEI `note`, `figure`): `"split": true`: met inside an open paragraph-like block, the element ends that block there, as a table or image does, and is read by its rule as if it stood outside; text after it, still in the paragraph's element, starts a new block (not `Continued`, with the paragraph's anchor). In a cell, an item, a heading or a value read for a rule it is read into it as before. A rule has `split` or `out_of_line`, not both |
| `"as": "break"` | rule kind | Phase 3 integration (Légifrance, AKN `br`/`eol`/`eop`, TEI `lb`, CDA and patent `br`, JATS `break`, DocBook `sbr`): an element that is a line break. Inside a block, a heading, an item or a cell it writes core's hard break, a `\n`, as go-fast-html does for `<br>`: white space on either side of it is dropped, two breaks give `\n\n`, and none is written at the start or end of a block. In a value read for a rule, a label or a metadata field it is a space; between blocks it writes nothing. The rule takes no keys; its element's attributes are counted as unread |
| `{"attr", "child"}` | value source | Phase 3 integration (CDA participants, patent `patcit@num`): a coded value in an attribute of a child of an element read for its text, which was dropped. See section 4 |
| (nothing dropped) | engine | Phase 3 integration. A waiting label that cannot be the marker of the next block (the block has one, or is a table or image) is a paragraph of its own before it; an ordered list's number gives way to the file's label; a block whose only text is its label (`<p><num>2</num></p>`) has the label as its text. The attributes of elements with no rule, and comments, inside a metadata value, a label or a stored element are notes where it stands. A metadata field that holds one value (`title`, `subject`, `language`, `created`, `modified`) takes the first; the others go to `Extra` under the field's name (`title` only when the rule has no `also_heading`: then they are headings) |
| `value` | `item` | Phase 3 integration (DITA `topicref@href`): the item's text when its element gives none of its own (an empty `topicref`, or one whose children are only other items); it then carries `Attrs["attribute"]`. It is not the `Marker` (a marker is the item's number or bullet) |
| (header cells) | `cell` with `header` | Phase 3 integration (AKN and HTML-like `th`): a row with no `header` of its own whose cells all match a cell rule whose `header` test is true counts in `HeaderRows` when it is among the leading rows (no body row before it) |
| (label inside a block) | `label` | Phase 3 (AKN `num` in a `quotedStructure` inside a paragraph, `num` in a table cell): a label met inside a paragraph or item that already has text, a marker or a waiting label, or inside a cell, is read as text of that block, after a space. It was the block's marker only if it had none, and was dropped otherwise. A label as the first thing in a block is its marker as before. An `out_of_line` note inside a label is a block of its own as inside a paragraph |
| `"as": "document"`, `media_type`, `when` | rule kind | Feeds: escaped HTML in an RSS `description` and Atom `content type="html"` was literal markup in a paragraph. The element's content is handed to the child extractor (section 3). `media_type`: `"text/html"`, `"application/xhtml+xml"`, or `{"attr": name, "map": {value: media type}}`; `when`: `"markup"` |
| `declares` | `match` entry | Feeds: an `rdf:RDF` root is RSS 1.0 only when it declares `http://purl.org/rss/1.0/`; other RDF stays generic |
| (late anchor) | engine | An anchor read from a child (`{"child": "guid"}`) that comes after blocks of its element were made gives those blocks `anchor_parent` too, and the element's embedded documents their `xml.parent_anchor` and name. In every profile; none of the twelve profiles before `feed` reads an anchor that way, so their output is unchanged |
| (one child, several uses) | engine | DILA case law: the root reads `META/META_COMMUN/ID` both as its `anchor` and as `Extra` (`"juri.id": {"child": "META/META_COMMUN/ID"}`). A child is read once: when several value sources of one rule name the same `child` selector, each gets the child's text (before, only the first did, and the next matching element went to the second). No profile before `dila-juri` names one child twice, so their output is unchanged |
| (late anchor, deeper child) | engine | An anchor read from a child deeper than one level (`META/META_COMMUN/ID`) now reaches the blocks after the elements between end (`META`, `META_COMMUN`): it was dropped with them. It is kept behind the anchors of nearer elements, which still win inside them. The output of every corpus file of the profiles before `dila-juri` is unchanged |
| (RFC 822 dates) | `metadata` `created`, `modified` | Feeds: a date in RFC 822 form with a numeric offset or `GMT`, `UT`, `Z` (`Mon, 02 Jan 2006 15:04:05 GMT`) fills the field; one with another zone name stays as written in `Extra` |
| `column` | `cell` | Invoices (CII, UBL): a line's values are nested elements at several depths (`SpecifiedTradeProduct/Name`, `SpecifiedLineTradeDelivery/BilledQuantity`), not cells in order. `"column": 4` (1 to 64): the cell's text goes to that column of the row it stands in, at any depth inside the row's element (or a `one_row` table's). A second element for the same column of one row follows the first after a `\n`. A column cell has no `colspan`/`rowspan`. Outside a row it makes no cell: its text is read where it stands |
| `rest` | `row`, `table` with `one_row` | Invoices: what a row holds that no column cell takes (a line's allowances, references, accounting cost) goes to this column (1 to 64), read as a cell's content is (blocks joined with `\n`), so that nothing of the line is lost or moved out of its row. Without `rest`, such content is read as it would be outside the row |
| `one_row` | `table` | Invoices: CII's `SpecifiedTradeSettlementHeaderMonetarySummation`, a UBL `AllowanceCharge` or `LegalMonetaryTotal` is one record with no row element. `"one_row": true`: the table's element is also its one row, filled by `column` cells (and `rest`); its row location is the table element's. A table whose only row is the profile's `header` (no row of the file) makes no block |
| `with` | `cell`, `block`, `inline` | Invoices: an amount means nothing without its `currencyID`, a quantity without its `unitCode`, an identifier without its `schemeID`. `"with": [{"attr": "unitCode"}]` (1 to 8 attributes of the element's own; no literal, `of`, `child`): their values are written after the element's text, each set off by a space (`<BilledQuantity unitCode="C62">2</BilledQuantity>` gives `2 C62`). An absent attribute writes nothing. It is text of the file, read from attributes, like an inline's `value` |
| `encoding`, `name` | `document` | Invoices: an attachment (CII `AttachmentBinaryObject`, UBL `EmbeddedDocumentBinaryObject`) is base64. `"encoding": "base64"`: the element's text, white space removed, is decoded and handed to `Options.ExtractChild`; with it `media_type` is `{"attr": "mimeCode"}` with no `map`, the attribute's value as written (none: `application/octet-stream`), and `when` is refused. `"name": {"attr": "filename"}` names the child (else the element's path, as for other document rules). The base64 is never written in the body: a child not extracted is listed (core's stub) with the same `xml.parent_*` keys and link. Text that is not base64 stays in the body as the fallback paragraph with `media_type`, and `input.stream_damaged` is raised. At most 64 MiB of base64 (`MaxAttachmentBytes`) is read; a longer attachment, or one past `MaxChildren`, is not handed over and not written, with `limit.exceeded`. The decoded bytes are taken from the decode budget as any child's |
| `pairs` | `table` | Invoices: a party, a payment term or a reference is a record of values whose meaning is the name of their element (`PostcodeCode`, `SpecifiedTaxRegistration/ID`); as bare paragraphs, rendered text could not tell `75001` from `FR`. `"pairs": "path"` (or `"local"`): the table's element is read as names and values, a row per value: its name, its text. The name is the file's, never the profile's: with `path` the path of local names starting with the table's own element (`SellerTradeParty/PostalTradeAddress/CountryID`), so that two tables of the same shape can be told apart in rendered text, with `local` the element's local name (`Volume`); the table's element itself, when it has text of its own, is named by its local name. Every attribute of these elements is a row of its own named `<name>/@<attribute>` (`SellerTradeParty/SpecifiedTaxRegistration/ID/@schemeID`), directly after its element's row (for an element holding elements, where it starts). An element holding elements makes no row; one with text and elements (mixed content) is one value, its children read into it. Each row's `RowLocs` entry is the value's element. Rules of elements inside the table are not applied, except `document` (its block comes before the table, as any block made inside a table); these elements are read, not counted as unmapped. Usually with `"header": ["name", "value"]`. Not with `one_row`, `rest` or the `cals` model |
| `rest_table` | `row`, `table` with `one_row` | Invoices: what a line holds beyond its column cells (allowances, references, a price's basis quantity) was joined in an `other` column with no names. `"rest_table": {"names": "path", "id": 1, "header": ["line id", "name", "value"]}`: every value inside the row's element that no column cell takes is a row of a second table written right after the table, read as in a `pairs` table (but names start below the row's element, which the `id` cell stands for; attributes as rows of their own, `RowLocs` on the value's element), in file order. `id` (optional, a column number): the first cell of each of these rows is the text of that column of the row it came from (the line's id as written). `header` (optional, 1 to 64 strings): the second table's header row, the profile's text. `names`: `path` or `local`. Not with `rest` |
| (YYYYMMDD) | `metadata` `created`, `modified` | Invoices: eight digits and nothing else are an ISO 8601 basic calendar date (UN/CEFACT format 102, as CII writes it) and fill the field |
| `value` | `inline` | Phase 6 (TEI `p/pb@n`, Formex `QUOT.START`/`QUOT.END`): an attribute of the inline's own element (`{"attr": "n"}` or `{"attr": "CODE", "codepoint": "hex"}`; no literal, `of`, `child` or `structure`) written where the element stands in the open block's text (or the value being read), as if it were the element's text: with `space`, set off by white space. The block's `Loc` still covers its element; this text is in the span as an attribute, not as character data. An absent attribute writes nothing. With `attrs` the record in the stored part has this text |
| `anchor_of` | value source, in `anchor` | PubMed: a record (`PubmedArticle`) is a `section` whose anchor is its PMID, read from a child (`{"child": "MedlineCitation/PMID"}`); its title, `ArticleTitle`, is deeper and later. `"anchor": {"anchor_of": "PubmedArticle"}` on the heading's rule gives the heading `Loc.Anchor` = the record's anchor, so that the PMID is the anchor of a block. The anchor must be known when the block is made (the PMID comes first); otherwise the block has none of its own and carries `anchor_parent` as before. Refused in any key but `anchor` and with any other key of a value source. No profile before `pubmed` uses it |
| (row attributes in `rest_table`) | `row` with `rest_table` | PubMed: an `Author` row carries `ValidYN` and `EqualContrib`, which no column cell can hold. The row element's own attributes are now the first rows the row gives its rest table, named `<row>/@<attribute>` (`Author/@ValidYN`), cited at the row element's start tag, with the row's id like the rest; they are no longer counted in `xml.unmapped_attributes`. The rows of CII and UBL lines carry no attributes in the dev corpus: the output of every dev file of the other profiles is unchanged |

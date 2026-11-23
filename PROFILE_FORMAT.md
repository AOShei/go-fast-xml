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

Load one with `gofastxml.LoadProfile`; a profile that does not load gives
an error naming the rule (`profile: rule 12 ("Chapitre/Titre"): ...`).

## 1. Choosing a profile

`match` is a list; a file matches when any entry does. An entry tests the
root element: `root` (local name), `namespace` (URI; absent means "no
namespace"), and optionally `doctype_public` (a prefix of the DOCTYPE
public identifier) and `attr` (attribute tests on the root, written as in
selectors: `"[@class~='topic/topic']"`).

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
| `table` | A table | `model`: `rows` (default) or `cals` |
| `row`, `cell` | In the `rows` model | `row`: `header` (`true` or an attribute test); `cell`: `header`, `colspan`, `rowspan` (attribute names) |
| `inline` | Text that stays inside its block | `style`: `bold`, `italic`, `superscript`, `subscript`, `code`; `link`: a value source for the address; `title` and `attrs` **(added)** |
| `label` | Its text becomes the `Marker` of the block it belongs to (the parent block, or the next block in the same container) | |
| `image` | An image block | `id` (value source for the address or identifier), `alt` (value source) |
| `note` | A `note` block | `type`: one of core's note types; `value` **(added)** |
| `metadata` | A metadata value; makes no block | `field`: `title`, `author`, `created`, `modified`, `language`, `keyword`, `subject`, `description`, or `extra` with `key`; `value` (value source, default the element's text); `also_heading`: a level, to also emit the title as a heading where it stands; `join` **(added)** |
| `part` | Starts a new part | `kind`: `body` or `chapter`; `name` (value source) |
| `stored` | The element's text goes to the stored part | `kind`: one of core's stored kinds |
| `transparent` | The element means nothing itself; its children are read by their own rules | |
| `skip` | Left out, counted in the info warning `xml.skipped_elements`. Refused in built-in profiles | |

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

A profile cannot add words to the text. Text comes only from the file.

## 4. Value sources

Where a rule takes a value:

| Form | Value |
|---|---|
| `"text"` | The literal string |
| `{"attr": "name"}` | The attribute of this element |
| `{"attr": "name", "of": "Ancestor"}` | The attribute of the nearest ancestor of that name |
| `{"child": "selector"}` | The text of the first matching child, which is then not emitted again. The selector is relative: `Theme/Titre` is a `Titre` in a `Theme` child |
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
- Inline styles and links become `Spans` when `Options.Spans` is set; in a
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
| `title` | `inline` with `link` | S6: the title of a link's target (`commentaireLien`). Core's `Span` has no field for it, so a link with a `title` or `attrs` gives a record in the stored part: a `note` (`stored`=`custom_property`, `name` the element's name) whose text is the title (with `type`=`attribute` and `attribute` naming it), else the link's text, and whose `Attrs` hold `href` and the rule's `attrs` |
| `attrs` | `inline` | The other attributes of a link (`type`, `audience`), in that record |
| `extra` | any rule but `label`, `skip` | Root attributes (`ID`, `spUrl`) into `Metadata.Extra` (C1) while the root stays `transparent` |
| `join` | `metadata` | The separator of repeated values of one `Extra` key (` > ` for the breadcrumb, C1); default a line break |
| `value` | `block`, `note` | A block whose text is an attribute (`RessourceWeb@URL`, C4); it carries `Attrs["attribute"]` |

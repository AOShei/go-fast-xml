# Profile `docbook`: DocBook 5 and DocBook 4

## What selects it

- DocBook 5: a root in the namespace `http://docbook.org/ns/docbook`
  (`book`, `article`, `chapter`, `section`, `refentry`, `table`, `para`
  and the other names listed in `docbookRoots` in
  `profile_docbook_test.go`).
- DocBook 4: the same roots with no namespace and a DOCTYPE public
  identifier starting `-//OASIS//DTD DocBook`, `-//KDE//DTD DocBook` or
  `-//FreeBSD//DTD DocBook`; and, with no DOCTYPE, the roots `chapter`,
  `appendix`, `colophon`, `preface`, `sect1`–`sect3`, `refentry`,
  `reference`. A DocBook 4 `book` or `article` with no DOCTYPE is not
  taken (JATS and BITS use those names).

**The profile is generated.** `profiles/docbook.json` is built from
`testdata/profiles/docbook/rules.json`, which holds each rule once in
DocBook 4 terms; the profile has every rule twice, once in the DocBook
namespace (prefix `db`, identifiers in `xml:id` instead of `id`) and
once as written. Edit the template, then run
`go test -run TestDocbookGenerated -docbook.update`; the test fails when
the two disagree.

## Mapping

| Element | Model |
|---|---|
| root `title`, `info/title`, `bookinfo/title`, `articleinfo/title` | `Metadata.Title` and a level-1 heading |
| `author`, `corpauthor` (in any `info`) | `Metadata.Authors`: the author's whole text, name fields set off by spaces (`Norman Walsh`; an `affiliation` or `email` inside `author` is part of that text) |
| `editor`, `othercredit`, `collab`, `pubdate`, `date`, `releaseinfo`, `edition`, `copyright`, `legalnotice`, `revhistory`, `publisher`, `biblioid`, `productname`, `productnumber`, `orgname` in an `info` | `Metadata.Extra` `db.*` |
| `keywordset/keyword`, `subjectterm` | `Metadata.Keywords` |
| `info/abstract` | section, blocks marked `db.in=abstract` |
| `info/subtitle` (not the root's) | paragraph `db.field=subtitle` |
| `set`, `book`, `part`, `reference`, `article`, `chapter`, `appendix`, `preface`, `colophon`, `dedication`, `acknowledgements`, `section`, `sect1`–`sect5`, `simplesect`, `topic`, `glossary`, `glossdiv`, `bibliography`, `bibliodiv`, `index`, `refentry`, `refsection`, `refsect1`–`3`, `refsynopsisdiv`, `qandadiv` | sections: headings nest by structure from 1 (the root's title) |
| `title` | heading, level by nesting; in `itemizedlist`, `orderedlist`, `variablelist`, `procedure`, `example`, `figure`, `table`, `equation`, `productionset`: caption; in an admonition or `sidebar`: paragraph `db.field=title`; in `formalpara`: the `Marker` of its paragraph |
| `bridgehead` | heading (level by nesting), `db.bridgehead` = `renderas` |
| `para`, `simpara` | paragraph |
| `blockquote`, `epigraph` | quote |
| `note`, `tip`, `important`, `caution`, `warning`, `danger` | their blocks marked `db.admonition` |
| `sidebar`, `example`, `informalexample`, `figure`, `informalfigure` | their blocks marked `db.box` |
| `itemizedlist`, `orderedlist`, `simplelist`, `procedure` (+ `step`, `substeps`), `calloutlist`, `segmentedlist`, `glosslist` | lists; `variablelist` a list whose item is the `term`, the `listitem` content following at the item's depth |
| `glossentry` | item: `glossterm` its text, `glossdef` paragraphs after it |
| `programlisting`, `screen`, `synopsis`, `literallayout` | code (verbatim), `Attrs["language"]` from `language`; `db.field` says which |
| `funcsynopsis`, `cmdsynopsis`, `classsynopsis`, `methodsynopsis`, `fieldsynopsis`, `production` | paragraph, fields set off by spaces, `db.field` the element |
| `table`, `informaltable` | table: CALS (`tgroup`, `colspec`, `spanspec`, `morerows`) or HTML (`tr`, `td`, `th`, `thead`); `title`/`caption` a caption |
| `mediaobject/imageobject/imagedata` | image, id = `fileref`; `textobject` and `alt` a paragraph (`db.field`) after it |
| `equation` | `mathphrase` and MathML a `formula` (plain) |
| `footnote` | footnote; inside a `para` it ends the paragraph there (`split`), and text after it is a new paragraph |
| `biblioentry`, `bibliomixed` | footnote, anchor = id, the entry's fields set off by spaces |
| `indexterm` | **stored**: a block in the stored part, `stored=custom_xml`, text `primary secondary ...`; not body text |
| `remark`, `annotation` | `note`, type `comment` |
| `xi:include` | paragraph whose text is the `href` (`Attrs["attribute"]=href`, `db.xpointer`); nothing is fetched |
| `xref` | link span to `linkend`, and a record in the stored part (`custom_property`, `href`) so that an `xref` with no text of its own is kept |
| `link`, `ulink`, `olink`, `biblioref`, `footnoteref`, `firstterm@linkend`, `glossterm@linkend` | link spans |
| `emphasis` (italic; `role="bold"`/`"strong"` bold), `firstterm`, `citetitle`, `foreignphrase`, `replaceable`, `superscript`, `subscript` | spans |
| `literal`, `code`, `tag`, `filename`, `command`, `option`, `parameter`, `varname`, `function`, `userinput`, `computeroutput`, `prompt`, `systemitem`, `classname`, `type`, `envar` and the other programming inlines | code spans |
| `guilabel`, `guimenu`, `keycap`, `menuchoice`, `acronym`, `phrase`, `quote`, ... | text with no style |

## Attrs keys

`db.field` (what a paragraph made from a non-paragraph element is),
`db.admonition`, `db.box`, `db.in` (`abstract`, `cover`, `legalnotice`,
`toc`), `db.list` (`variablelist`, `segmentedlist`, `procedure`),
`db.label` (`label` attribute of a chapter, appendix or section),
`db.role`, `db.class`, `db.numeration`, `db.mark`, `db.condition`,
`db.format`, `db.math`, `db.xpointer`, `db.xreflabel`, `db.otherterm`,
`db.bridgehead`, `db.arearefs`, `db.annotates`.

## How to cite a block

`Loc.Offset`/`Length`, and `Loc.Anchor` = the block element's `xml:id`
(DocBook 5) or `id` (DocBook 4); headings and other blocks with no id
carry their section's (chapter's, figure's...) id in
`Attrs["anchor_parent"]`, which is what `xref@linkend` points at.

## Limits of the format met here

- A `footnote`, table, figure, example, admonition, `blockquote` or
  verbatim block inside a `para` ends the paragraph there (`split`, or
  the engine's rule for tables and images) and keeps its own kind and
  anchor; text after it is a new paragraph. A list inside a `para` is
  still joined into the paragraph's text, one item per line.
- DocBook entities declared in an external DTD (KDE's `&kate;`) are kept
  as written; the reader expands those of the internal subset.

## Left to generic reading

`choose`, `when`, `otherwise`, `sequence`, `message`, `sup` inside
`fieldsynopsis/initializer` in the xslTNG reference (XSLT written as
DocBook content in another vocabulary): generic reading keeps their text
in the paragraph. Unread attributes are counted: presentation
(`@spacing`, `@frame`, `@linenumbering`, `@role` of inlines), `tag@class`,
`keycombo@action`, `indexterm@class`, the root's `version`.

## Versions

Samples: DocBook 5.0/5.1/5.2 (Definitive Guide 5, xslTNG, nixpkgs,
FreeBSD), DocBook 4.1.2–4.5 (TLDP, KDE, GNOME, Definitive Guide 4).
Not seen: SGML DocBook (not XML), DocBook 5 `assembly`/`resource`, `msgset`,
`task`, `qandaset` with `qandadiv`, `entrytbl`, `videodata`.

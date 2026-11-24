# Profile `feed`: RSS and Atom web feeds

## What selects it

The root `rss` with no namespace (RSS 0.91, 0.92, 2.0); the root `feed`
in `http://www.w3.org/2005/Atom` (Atom 1.0); the root `RDF` in
`http://www.w3.org/1999/02/22-rdf-syntax-ns#` only when it declares
`http://purl.org/rss/1.0/` (RSS 1.0, `declares`). Any other RDF file is
read generically. RSS 1.0's namespace is read as no namespace
(`namespace_aliases`), so one set of rules reads RSS 2.0 and 1.0; the
`https://` spellings of the Atom, Dublin Core, content, Slash,
syndication, Media RSS and CommentAPI namespaces met in feeds are read as
their `http://` names. Built-in profiles are tried in the order of their
file names; `feed` comes after `dita` and before `formex`, and no other
built-in profile's selector names these roots.

## Feed level

| Element | Model |
|---|---|
| `channel/title`, `feed/title` | `Metadata.Title` and a level-1 heading (an Atom title of type `html` stays as written) |
| `channel/description`, `itunes:summary`, `googleplay:description` | `when: markup`: a child document (`text/html`), else a paragraph |
| `feed/subtitle` | by `type`: `html`, `xhtml` a child document; otherwise a paragraph |
| `channel/language`, `dc:language` | `Metadata.Language` |
| `lastBuildDate`, `dc:date`, Atom `updated` | `Metadata.Modified` (RFC 3339, or RFC 822 with a numeric offset or GMT/UT/Z; otherwise as written in `Extra["modified"]`) |
| `channel/pubDate` | `Metadata.Created`, same rule |
| `channel/category`, `itunes:category@text`, Atom `category@term` | `Metadata.Keywords` |
| `dc:creator`, `itunes:author`, `googleplay:author`, Atom `author/name` | `Metadata.Authors` |
| `link`, `generator` (+ Atom `@uri`, `@version`), `copyright`, `dc:rights`, Atom `rights`, `managingEditor`, `webMaster`, `docs`, `ttl`, `dc:publisher`, `dc:source`, `sy:update*`, `itunes:keywords`, `explicit`, `type`, `new-feed-url`, `owner/name`, `owner/email`, Atom `id`, `icon`, `logo`, `contributor/name`, `author/uri`, `author/email` | `Metadata.Extra` under `feed.*` keys (as written) |
| `image` (RSS) | an image block: id `url`, alt `title`, `Attrs` `feed.link`, `feed.width`, `feed.height`, `feed.description` |
| `textInput` (RSS 2.0), `textinput` (RSS 0.91, 1.0) | a paragraph whose text is its `title`, `Attrs` `feed.field=textInput`, `feed.description`, `feed.name`, `feed.link`; anchor `rdf:about` |
| `itunes:image@href`, `media:thumbnail@url` | image blocks |
| `channel/items/rdf:Seq/rdf:li@rdf:resource` (RSS 1.0) | attribute notes, `feed.field=items` |

## Items and entries

| Element | Model |
|---|---|
| `item`, `entry` | a section; anchor `guid` (RSS 2.0), `rdf:about` (RSS 1.0), Atom `id`: every block of the item carries it as `Attrs["anchor_parent"]` (also those made before a late `guid`). An item that makes no block (only a `link`, a `guid`, an `rdf:about`) writes what it read for its heading and anchor as the blocks their elements' rules make (`link`, `guid` paragraphs), an attribute's value as an attribute note |
| `item/title`, `entry/title` | a level-2 heading; RSS titles as written. An Atom title of type `html` or `xhtml` is a heading holding the text as written (for `xhtml`, its text content) with `Attrs["media_type"]`; no child is made for a title |
| on the heading | `feed.link`, `feed.published` (`pubDate`, Atom `published`), `feed.updated`, `feed.date` (`dc:date`), `feed.guid_is_permalink`, `feed.itunes_duration`, `_episode`, `_season`, `_episode_type`, `_explicit`, `_block`, `feed.georss_point`, `feed.geo_lat`, `feed.geo_long` (the first of each; a second is read by the rule below), and `children` |
| `description`, `content:encoded`, `itunes:summary`, `dc:description`, `media:description` | `when: markup`: a child document (`text/html`), else a paragraph; `media:description type="html"` is always a child |
| Atom `content`, `summary`, `rights` | by `type`: `html` a child (`text/html`), `xhtml` a child (`application/xhtml+xml`), `text` or none a paragraph as written (`feed.type`); `content` with `src` an attribute note (`feed.field=content`), nothing fetched |
| `author`, `comments`, `category` (+ `feed.domain`), `source` (+ `feed.url`), extra `link`, `guid`, `pubDate`, `dc:*`, `itunes:title`, `author`, `keywords`, `media:title`, `credit`, `keywords`, `rating`, `slash:*`, `wfw:commentRss`, `thr:total`, `podcast:person` (+ role, group, href, img), `podcast:funding` | paragraphs with `Attrs["feed.field"]` the element's name (as JATS marks `jats.field`) |
| Atom `author`, `contributor` | a paragraph whose text is `name`, `feed.uri`, `feed.email` |
| `itunes:subtitle`, `media:text` | paragraphs (`feed.field`) |
| `enclosure` | attribute note: `url`, `feed.type`, `feed.length` |
| Atom `link` (also `atom:link` in RSS) | attribute note: `href`, `feed.rel`, `feed.type`, `feed.title`, `feed.hreflang`, `feed.length` |
| Atom `category` | attribute note: `term`, `feed.scheme`, `feed.label` |
| `media:content` | attribute note: `url`, `feed.type`, `medium`, `width`, `height`, `file_size`, `duration`, `lang`; its children by their rules |
| `podcast:transcript`, `podcast:chapters`, `podcast:socialInteract` | attribute notes (`url` / `uri`, type, language, rel, protocol) |
| `psc:chapters` | a list; each `psc:chapter` an item whose text is its `title`, `feed.start`, `feed.href`, `feed.image` |

## Embedded documents

See the README, "Embedded documents". The child's name is
`item/description#<guid>`; the item's heading carries `children`; each
child carries `xml.parent_offset`, `xml.parent_length`,
`xml.parent_anchor`, `xml.parent_element`. Without a child extractor,
the text is kept as a paragraph with `Attrs["media_type"]`. A comment
inside the element is the child's (in the bytes handed over) and no note
of the feed; when the text stays in the body, it is a note after that
paragraph, as anywhere else.

## Left to generic reading

`skipDays`, `skipHours`, `cloud`, `rating`,
Atom `source`, OpenSearch, PRISM, arXiv, GDACS (its `gdacs:title` and
`gdacs:description` too: an extension's vocabulary, read as the rest of
it is), legislation.gov.uk
facets, WordPress `post-id`, Creative Commons `license`, `admin:`,
`podcast:` elements other than those above, and any other extension:
text as inferred paragraphs, attributes as notes, counted in
`xml.unmapped_elements`. Nothing is dropped.

## Dev set (76 files)

Without a child extractor: 0 errors, 0 Validate failures; blocks:
heading 5,844, image 1,258, list_item 2,057, note 7,128, paragraph 28,843;
children listed: `text/html` 3,279, `application/xhtml+xml` 1; elements
kept as paragraphs by `when: markup` 4,468 (counted by element name at the
block's offset). With a child extractor, the same 3,280 children are
extracted and paragraphs fall to 25,563. Unmapped elements: 8,655 in all,
mostly GDACS (188 per element, `gdacs:title` and `gdacs:description`
among them), arXiv `announce_type` (446), legislation.gov.uk `facetYear`
(241) and PRISM.

`TestFeedOracles` passes over the 76 files in both modes: (a) the body's
text in order, with each child's text at its place, is the file's
character data in order, apart from the listed metadata and attribute
values; (b) every block's text is in the file at its offsets, a fallback
paragraph's in full (the source unescaped). One file is cut off in an
item (`unexpected EOF` for `encoding/xml`): (a) compares its text up to
there.

## Held-out set (25 files)

0 errors, 0 Validate failures; 24 read with `feed`, 1 not. Without a child extractor: heading 2,895, image
1,772, note 12,367, paragraph 19,282; `text/html` children 4,762. With
one, all 4,762 are extracted and paragraphs fall to 14,520. Unmapped
elements: 6,570.

## Not tested

RSS 0.90; Atom 0.3 (`http://purl.org/atom/ns#`, read generically);
`xhtml` content whose namespace prefixes are declared outside the element
(handed over without them).

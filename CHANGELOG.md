# Changelog

## Unreleased

- Streaming XML scanner (`internal/xmlscan`), adapted from go-fast-odf's:
  namespace URIs, token offsets, comments, processing instructions and the
  document type declaration as tokens, general entities expanded within a
  bound on nesting and on the bytes produced, recovery from damaged markup.
- `internal/charset` and `internal/entity` copied from go-fast-html.

package charset

import "bytes"

// prescan is the HTML standard's "prescan a byte stream to determine its
// encoding" (section 13.2.3.2), returning the label declared rather than
// the encoding, so that a label this package does not decode can be
// reported. It reports false when nothing is declared.
func prescan(b []byte) (string, bool) {
	for i := 0; i < len(b); {
		rest := b[i:]
		switch {
		case bytes.HasPrefix(rest, []byte("<!--")):
			// The "-->" may share its dashes with the "<!--".
			end := bytes.Index(b[i+2:], []byte("-->"))
			if end < 0 {
				return "", false
			}
			i += 2 + end + 3
		case len(rest) >= 6 && bytes.EqualFold(rest[:5], []byte("<meta")) && (isSpace(rest[5]) || rest[5] == '/'):
			i += 6
			label, ok, next := metaCharset(b, i)
			i = next
			if ok {
				return label, true
			}
		case len(rest) >= 2 && rest[0] == '<' && (isLetter(rest[1]) || rest[1] == '/' && len(rest) >= 3 && isLetter(rest[2])):
			i++
			for i < len(b) && !isSpace(b[i]) && b[i] != '>' {
				i++
			}
			for {
				var ok bool
				if _, _, i, ok = getAttr(b, i); !ok {
					break
				}
			}
			if i < len(b) && b[i] == '>' {
				i++
			}
		case bytes.HasPrefix(rest, []byte("<!")) || bytes.HasPrefix(rest, []byte("</")) || bytes.HasPrefix(rest, []byte("<?")):
			end := bytes.IndexByte(rest, '>')
			if end < 0 {
				return "", false
			}
			i += end + 1
		default:
			i++
		}
	}
	return "", false
}

// metaCharset reads the attributes of a <meta> tag from b[i:] and returns
// the label it declares, if any, and where scanning resumes.
func metaCharset(b []byte, i int) (label string, ok bool, next int) {
	seen := map[string]bool{}
	gotPragma := false
	needPragma := 0 // 0 null, 1 false, 2 true
	charset := ""
	have := false
	for {
		name, value, n, more := getAttr(b, i)
		i = n
		if !more {
			break
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		switch name {
		case "http-equiv":
			if value == "content-type" {
				gotPragma = true
			}
		case "content":
			if !have {
				if c, found := charsetFromContent(value); found {
					charset, have = c, true
					needPragma = 2
				}
			}
		case "charset":
			charset, have = value, true
			needPragma = 1
		}
	}
	if i < len(b) && b[i] == '>' {
		i++
	}
	if needPragma == 0 || needPragma == 2 && !gotPragma || !have {
		return "", false, i
	}
	charset = string(bytes.Trim([]byte(charset), "\t\n\f\r "))
	if charset == "" {
		return "", false, i
	}
	if len(charset) > 64 {
		charset = charset[:64]
	}
	return charset, true, i
}

// getAttr is the prescan's "get an attribute" algorithm. It returns the
// lowercased name and value, the position after them, and false when there
// is no further attribute.
func getAttr(b []byte, i int) (name, value string, next int, ok bool) {
	for i < len(b) && (isSpace(b[i]) || b[i] == '/') {
		i++
	}
	if i >= len(b) || b[i] == '>' {
		return "", "", i, false
	}
	var nb, vb []byte
	for ; i < len(b); i++ {
		c := b[i]
		if c == '=' && len(nb) > 0 {
			i++
			goto value
		}
		if isSpace(c) {
			for i < len(b) && isSpace(b[i]) {
				i++
			}
			if i < len(b) && b[i] == '=' {
				i++
				goto value
			}
			return lower(nb), "", i, true
		}
		if c == '/' || c == '>' {
			return lower(nb), "", i, true
		}
		nb = append(nb, c)
	}
	return lower(nb), "", i, true
value:
	for i < len(b) && isSpace(b[i]) {
		i++
	}
	if i >= len(b) {
		return lower(nb), "", i, true
	}
	if q := b[i]; q == '"' || q == '\'' {
		i++
		for i < len(b) && b[i] != q {
			vb = append(vb, b[i])
			i++
		}
		if i < len(b) {
			i++
		}
		return lower(nb), lower(vb), i, true
	}
	if b[i] == '>' {
		return lower(nb), "", i, true
	}
	for i < len(b) && !isSpace(b[i]) && b[i] != '>' {
		vb = append(vb, b[i])
		i++
	}
	return lower(nb), lower(vb), i, true
}

// charsetFromContent is the standard's "extract a character encoding from
// a meta element" for the value of a content attribute.
func charsetFromContent(s string) (string, bool) {
	b := []byte(s)
	for {
		k := bytes.Index(b, []byte("charset"))
		if k < 0 {
			return "", false
		}
		b = b[k+7:]
		j := 0
		for j < len(b) && isSpace(b[j]) {
			j++
		}
		if j >= len(b) || b[j] != '=' {
			b = b[j:]
			continue
		}
		b = b[j+1:]
		break
	}
	j := 0
	for j < len(b) && isSpace(b[j]) {
		j++
	}
	b = b[j:]
	if len(b) == 0 {
		return "", false
	}
	if q := b[0]; q == '"' || q == '\'' {
		end := bytes.IndexByte(b[1:], q)
		if end < 0 {
			return "", false
		}
		return string(b[1 : 1+end]), true
	}
	end := 0
	for end < len(b) && !isSpace(b[end]) && b[end] != ';' {
		end++
	}
	return string(b[:end]), true
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

func isLetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

// lower lowercases the ASCII letters of b only, as the HTML and Encoding
// standards match names and labels: Unicode lowercasing would make
// U+212A KELVIN SIGN a k.
func lower(b []byte) string {
	out := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

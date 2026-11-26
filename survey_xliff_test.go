package gofastxml

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var surveyFlag = flag.String("survey", "", "dev corpus set for TestSurveyTranslation (xml-xliff, xml-tmx)")

// TestSurveyTranslation lists a dev set's roots, namespaces, versions,
// element paths and attributes with counts (-survey xml-xliff).
func TestSurveyTranslation(t *testing.T) {
	set := *surveyFlag
	if set == "" {
		t.Skip("-survey not set")
	}
	dir := filepath.Join(corpusRoot(), "dev", set)
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Skip(err)
	}
	roots, els, atts := map[string]int{}, map[string]int{}, map[string]int{}
	short := func(n xml.Name) string {
		s := n.Space
		switch {
		case strings.HasPrefix(s, "urn:oasis:names:tc:xliff:document:"):
			s = "x" + strings.TrimPrefix(s, "urn:oasis:names:tc:xliff:document:")
		case s == "":
		default:
			if i := strings.LastIndexAny(s, ":/"); i >= 0 && i < len(s)-1 {
				s = s[i+1:]
			}
		}
		if s == "" {
			return n.Local
		}
		return "{" + s + "}" + n.Local
	}
	for _, e := range ents {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(data) > 8<<20 {
			continue
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		d.Strict = false
		d.CharsetReader = func(string, io.Reader) (io.Reader, error) { return nil, fmt.Errorf("charset") }
		var stack []string
		for {
			tok, err := d.Token()
			if err != nil {
				break
			}
			switch v := tok.(type) {
			case xml.StartElement:
				name := short(v.Name)
				if len(stack) == 0 {
					ver := ""
					for _, a := range v.Attr {
						if a.Name.Local == "version" {
							ver = a.Value
						}
					}
					roots[v.Name.Space+" "+v.Name.Local+" v="+ver]++
				}
				p := name
				if len(stack) > 0 {
					p = stack[len(stack)-1] + "/" + name
				}
				els[p]++
				for _, a := range v.Attr {
					if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
						continue
					}
					atts[name+"@"+short(a.Name)]++
				}
				stack = append(stack, name)
			case xml.EndElement:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	dump := func(title string, m map[string]int) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Println("==", title)
		for _, k := range keys {
			fmt.Printf("%7d %s\n", m[k], k)
		}
	}
	dump("roots", roots)
	dump("elements", els)
	dump("attributes", atts)
}

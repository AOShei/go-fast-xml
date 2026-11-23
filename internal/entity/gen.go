//go:build ignore

// gen writes entities.txt from the WHATWG entities.json
// (https://html.spec.whatwg.org/entities.json), whose keys are "&name;"
// or, for the legacy names, "&name", and whose values give the code
// points. Usage: go run gen.go entities.json entities.txt
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var in map[string]struct {
		Codepoints []int `json:"codepoints"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		panic(err)
	}
	var full, legacy []string
	for k := range in {
		name := strings.TrimPrefix(k, "&")
		if n, ok := strings.CutSuffix(name, ";"); ok {
			full = append(full, n)
		} else {
			legacy = append(legacy, name)
		}
	}
	sort.Strings(full)
	sort.Strings(legacy)
	var b strings.Builder
	b.WriteString(`# HTML named character references, from the WHATWG HTML Living Standard,
# section 13.5 "Named character references" (entities.json). Each pair is a
# name without its "&" and ";" and the code points it stands for, in hex,
# joined with "+" where there are two. Every name is valid with a ";". The
# names on the "legacy" lines are also matched without one.
# gen.go writes this file from entities.json; see NOTICE.
`)
	line := 0
	emit := func(s string) {
		if line > 0 && line+1+len(s) > 78 {
			b.WriteByte('\n')
			line = 0
		}
		if line > 0 {
			b.WriteByte(' ')
			line++
		}
		b.WriteString(s)
		line += len(s)
	}
	for _, n := range full {
		var cps []string
		for _, c := range in["&"+n+";"].Codepoints {
			cps = append(cps, fmt.Sprintf("%X", c))
		}
		emit(n + " " + strings.Join(cps, "+"))
	}
	b.WriteByte('\n')
	line = 0
	for i, n := range legacy {
		if i%10 == 0 {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("legacy")
		}
		b.WriteString(" " + n)
	}
	b.WriteByte('\n')
	if err := os.WriteFile(os.Args[2], []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
}

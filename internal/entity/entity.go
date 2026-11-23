// Package entity resolves HTML character references: the named references
// of the WHATWG HTML standard (section 13.5, embedded from entities.txt)
// and the numeric ones with the standard's replacement rules.
package entity

import (
	_ "embed"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/AOShei/go-fast-xml/internal/charset"
)

//go:generate go run gen.go entities.json entities.txt

//go:embed entities.txt
var table string

// MaxName is the length of the longest name, without "&" and ";".
const MaxName = 31

var (
	once  sync.Once
	names map[string]string // "amp;" and, for the legacy names, "amp"
)

func load() {
	names = make(map[string]string, 2400)
	for line := range strings.Lines(table) {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if f[0] == "legacy" {
			for _, n := range f[1:] {
				names[n] = names[n+";"]
			}
			continue
		}
		for i := 0; i+1 < len(f); i += 2 {
			var sb strings.Builder
			for cp := range strings.SplitSeq(f[i+1], "+") {
				v, err := strconv.ParseUint(cp, 16, 32)
				if err != nil {
					panic("entity: bad table entry " + f[i])
				}
				sb.WriteRune(rune(v))
			}
			names[f[i]+";"] = sb.String()
		}
	}
}

// Len returns the number of names in the table, legacy forms included.
func Len() int {
	once.Do(load)
	return len(names)
}

// Named finds the longest named character reference at the start of s,
// which follows an "&". It returns what the reference stands for, the
// bytes of s it takes (its ";" included when it has one), and whether
// that final ";" was there.
func Named(s []byte) (repl string, n int, semicolon, ok bool) {
	once.Do(load)
	run := 0
	for run < len(s) && run < MaxName+1 && isAlnum(s[run]) {
		run++
	}
	if run < len(s) && s[run] == ';' {
		if r, found := names[string(s[:run+1])]; found {
			return r, run + 1, true, true
		}
	}
	for k := min(run, MaxName); k > 0; k-- {
		if r, found := names[string(s[:k])]; found {
			return r, k, false, true
		}
	}
	return "", 0, false, false
}

// Lookup returns what the reference "&name" stands for, name including
// its ";" if it has one.
func Lookup(name string) (string, bool) {
	once.Do(load)
	r, ok := names[name]
	return r, ok
}

// Numeric returns the character a numeric character reference to code
// point v stands for, following the standard's "numeric character
// reference end state": zero, a surrogate or a value beyond U+10FFFF is
// U+FFFD, and a C1 control is the windows-1252 character of its byte.
// Other noncharacters and controls are returned as they are.
func Numeric(v int64) rune {
	switch {
	case v == 0 || v > utf8.MaxRune || v >= 0xD800 && v <= 0xDFFF:
		return utf8.RuneError
	}
	if r, ok := charset.C1(rune(v)); ok {
		return r
	}
	return rune(v)
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z'
}

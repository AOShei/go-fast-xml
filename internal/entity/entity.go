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

// classicNames are the 252 character entities of HTML 4 (its Latin-1,
// symbol and special sets), the names XML vocabularies took from the same
// ISO sets and that mean one thing everywhere.
const classicNames = `nbsp iexcl cent pound curren yen brvbar sect uml copy ordf laquo not shy reg macr deg plusmn
sup2 sup3 acute micro para middot cedil sup1 ordm raquo frac14 frac12 frac34 iquest Agrave Aacute Acirc Atilde Auml
Aring AElig Ccedil Egrave Eacute Ecirc Euml Igrave Iacute Icirc Iuml ETH Ntilde Ograve Oacute Ocirc Otilde Ouml times
Oslash Ugrave Uacute Ucirc Uuml Yacute THORN szlig agrave aacute acirc atilde auml aring aelig ccedil egrave eacute
ecirc euml igrave iacute icirc iuml eth ntilde ograve oacute ocirc otilde ouml divide oslash ugrave uacute ucirc uuml
yacute thorn yuml
fnof Alpha Beta Gamma Delta Epsilon Zeta Eta Theta Iota Kappa Lambda Mu Nu Xi Omicron Pi Rho Sigma Tau Upsilon Phi Chi
Psi Omega alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigmaf sigma tau
upsilon phi chi psi omega thetasym upsih piv bull hellip prime Prime oline frasl weierp image real trade alefsym larr
uarr rarr darr harr crarr lArr uArr rArr dArr hArr forall part exist empty nabla isin notin ni prod sum minus lowast
radic prop infin ang and or cap cup int there4 sim cong asymp ne equiv le ge sub sup nsub sube supe oplus otimes perp
sdot lceil rceil lfloor rfloor lang rang loz spades clubs hearts diams
quot amp lt gt OElig oelig Scaron scaron Yuml circ tilde ensp emsp thinsp zwnj zwj lrm rlm ndash mdash lsquo rsquo
sbquo ldquo rdquo bdquo dagger Dagger permil lsaquo rsaquo euro`

var (
	classicOnce sync.Once
	classic     map[string]bool
)

// Classic returns what the HTML 4 character entity "&name;" stands for;
// name has no "&" and no ";". A name outside that set is not found, even
// when the HTML standard has one of that name: such names are not the
// same in every vocabulary.
func Classic(name string) (string, bool) {
	classicOnce.Do(func() {
		classic = make(map[string]bool, 256)
		for _, n := range strings.Fields(classicNames) {
			classic[n] = true
		}
	})
	if !classic[name] {
		return "", false
	}
	return Lookup(name + ";")
}

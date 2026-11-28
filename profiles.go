package gofastxml

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"path"
	"sort"
	"sync"

	"github.com/AOShei/go-fast-core/xmlscan"
	"github.com/AOShei/go-fast-xml/internal/engine"
	"github.com/AOShei/go-fast-xml/internal/generic"
	"github.com/AOShei/go-fast-xml/internal/profile"
	"github.com/AOShei/go-fast-xml/internal/xmlread"
)

// Profile maps the elements of one XML schema to the shared model. See
// PROFILE_FORMAT.md. A Profile is immutable and safe for concurrent use.
type Profile struct {
	p *profile.Profile
}

// Name returns the profile's name, as Metadata.Extra["xml.profile"] gives it.
func (p *Profile) Name() string { return p.p.Name }

// Title returns the profile's description.
func (p *Profile) Title() string { return p.p.Title }

// LoadProfile reads and validates a profile. An error names the rule it
// is about.
func LoadProfile(r io.Reader) (*Profile, error) {
	p, err := profile.Load(r, false)
	if err != nil {
		return nil, err
	}
	return &Profile{p}, nil
}

// ProfileNone is the name that forces generic reading.
const ProfileNone = "none"

// Warning codes of profile reading (all info).
const (
	// WarnUnmappedElements: elements the profile has no rule for were read
	// generically; the message names them with counts.
	WarnUnmappedElements = engine.WarnUnmappedElements
	// WarnUnmappedAttributes: attributes of elements the profile maps that
	// no rule reads; the message names them (element@attribute) with
	// counts. Their values are not in the output.
	WarnUnmappedAttributes = engine.WarnUnmappedAttributes
	// WarnSkippedElements: elements a user profile's skip rules left out.
	WarnSkippedElements = engine.WarnSkipped
)

// ExtraProfile is the Metadata.Extra key naming the profile used.
const ExtraProfile = "xml.profile"

// AttrAnchorParent is the Attrs key of the identifier of the nearest
// enclosing element a profile names with "anchor".
const AttrAnchorParent = engine.AttrAnchorParent

// AttrHeaderSource is "profile" on a table whose header row is the fixed
// text of the profile's "header", not text of the file.
const AttrHeaderSource = engine.AttrHeaderSource

//go:embed profiles/*.json
var profileFiles embed.FS

var builtin = sync.OnceValue(func() []*Profile {
	entries, err := profileFiles.ReadDir("profiles")
	if err != nil {
		panic(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var ps []*Profile
	for _, n := range names {
		data, err := profileFiles.ReadFile(path.Join("profiles", n))
		if err != nil {
			panic(err)
		}
		p, err := profile.Load(bytes.NewReader(data), true)
		if err != nil {
			panic(fmt.Sprintf("gofastxml: built-in profile %s: %v", n, err))
		}
		ps = append(ps, &Profile{p})
	}
	return ps
})

// BuiltinProfiles returns the built-in profiles in the order they are
// tried.
func BuiltinProfiles() []*Profile { return append([]*Profile(nil), builtin()...) }

// BuiltinProfile returns the built-in profile of that name, or nil.
func BuiltinProfile(name string) *Profile {
	for _, p := range builtin() {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// forced returns the profile Extractor.Profile names; none is (nil, true).
func (e *Extractor) forced() (*Profile, bool, error) {
	switch e.Profile {
	case "":
		return nil, false, nil
	case ProfileNone:
		return nil, true, nil
	}
	for _, p := range e.Profiles {
		if p != nil && p.Name() == e.Profile {
			return p, true, nil
		}
	}
	if p := BuiltinProfile(e.Profile); p != nil {
		return p, true, nil
	}
	return nil, false, fmt.Errorf("gofastxml: no profile named %q", e.Profile)
}

// rootPath is the path of the root element, for matching profiles.
type rootPath struct{ el *xmlread.Element }

func (p rootPath) Len() int { return 1 }
func (p rootPath) Name(int) profile.QName {
	return profile.QName{URI: p.el.URI, Local: p.el.Local}
}
func (p rootPath) Attr(_ int, q profile.QName) (string, bool) { return p.el.Attr(q.URI, q.Local) }

// Declares reports whether the root declares the namespace uri.
func (p rootPath) Declares(uri string) bool {
	for _, a := range p.el.Attrs {
		if a.URI == xmlscan.XMLNSNamespace && a.Value == uri {
			return true
		}
	}
	return false
}

// dispatch hands the events to the generic reader until the root element
// comes, then to the profile that matches it, if any.
type dispatch struct {
	x       *Extractor
	out     *generic.Out
	g       *generic.Generic
	eng     *engine.Engine
	h       xmlread.Handler
	opts    engine.Options
	forced  *Profile
	none    bool
	started bool
	used    *Profile
}

func (d *dispatch) StartElement(r *xmlread.Reader, e *xmlread.Element) {
	if !d.started {
		d.started = true
		p := d.forced
		if p == nil && !d.none {
			pub := ""
			if dt := r.Report.Doctype; dt != nil {
				pub = dt.PublicID
			}
			for _, c := range append(append([]*Profile(nil), d.x.Profiles...), builtin()...) {
				if c != nil && c.p.MatchesRoot(rootPath{e}, pub) {
					p = c
					break
				}
			}
		}
		if p != nil {
			d.g.Finish(e.Start)
			d.used = p
			d.eng = engine.New(p.p, d.out, d.opts)
			d.h = d.eng
		}
	}
	d.h.StartElement(r, e)
}

func (d *dispatch) EndElement(r *xmlread.Reader, e *xmlread.Element, start, end int64) {
	d.h.EndElement(r, e, start, end)
}

func (d *dispatch) Text(r *xmlread.Reader, t []byte, start, end int64) { d.h.Text(r, t, start, end) }

func (d *dispatch) Comment(r *xmlread.Reader, t []byte, final bool, start, end int64) {
	d.h.Comment(r, t, final, start, end)
}

func (d *dispatch) Stop() bool { return d.h.Stop() }

func (d *dispatch) Finish(end int64) {
	if d.eng != nil {
		d.eng.Finish(end)
		return
	}
	d.g.Finish(end)
}

func (d *dispatch) Counts() (controls int) {
	c := d.g.Counts()
	if d.eng != nil {
		c += d.eng.Counts()
	}
	return c
}

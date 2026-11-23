// Command gofastxml extracts XML documents into the shared document model
// of the go-fast libraries and prints them as JSON, Markdown or plain
// text.
//
//	gofastxml [flags] file-or-directory...
//
// A directory stands for the .xml files in it and below it. With one input
// the JSON is indented; with several, each document is one line. Only the
// documents go to standard output: everything said about them goes to
// standard error, and the exit status is 1 if any input could not be
// read.
//
// Its shape follows go-fast-odf's command gofastodf (same author, MIT
// License).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/AOShei/go-fast-core"
	"github.com/AOShei/go-fast-xml"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command: it reads args and writes documents to stdout and
// messages to stderr, and returns the exit status.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gofastxml", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var (
		format     = flags.String("format", "json", "what to print: json, markdown or text")
		output     = flags.String("o", "", "write to this file instead of standard output")
		attributes = flags.Bool("attributes", true, "make a note of each attribute value at its element's place; -attributes=false leaves them out (for data-style XML)")
		maxInput   = flags.Int64("max-input-bytes", 0, "size of a file that is read (0: the default, 4 GiB; -1: no limit)")
		maxBytes   = flags.Int64("max-decoded-bytes", 0, "bytes entity expansion may produce (0: the default, 1 GiB, and never more than 1 MiB plus 8 times the file; -1: no limit but that)")
		maxText    = flags.Int64("max-text-bytes", 0, "bytes of text a document may hold (0: the default, 256 MiB; -1: no limit)")
		quiet      = flags.Bool("quiet", false, "say nothing about files that were read; failures are still reported")
		version    = flags.Bool("version", false, "print the version and exit")
	)
	// The render flags shared by every go-fast command (COMMAND_FLAGS.md).
	renderFlags := gofastcore.AddRenderFlags(flags)
	flags.Lookup("stored").Usage += " (this library writes markup_comment, the file's <!-- --> comments)"
	flags.Usage = func() {
		fmt.Fprintln(stderr, "usage: gofastxml [flags] file-or-directory...")
		fmt.Fprintln(stderr, "Extracts XML documents. A directory stands for the .xml files in it and below it.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *version {
		fmt.Fprintln(stdout, "gofastxml", moduleVersion())
		return 0
	}
	fail := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "gofastxml: "+format+"\n", args...)
		return 2
	}
	if flags.NArg() == 0 {
		flags.Usage()
		return 2
	}
	if *format != "json" && *format != "markdown" && *format != "text" {
		return fail("unknown format %q: json, markdown or text", *format)
	}
	var opts gofastcore.Options
	opts.Limits.MaxInputBytes, opts.Limits.MaxDecodedBytes, opts.Limits.MaxTextBytes = *maxInput, *maxBytes, *maxText
	extractor := &gofastxml.Extractor{}
	if !*attributes {
		extractor.Attributes = gofastxml.AttributesNone
	}

	var inputs []string
	status := 0
	for _, arg := range flags.Args() {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(stderr, "gofastxml: %v\n", err)
			status = 1
			continue
		}
		if !info.IsDir() {
			inputs = append(inputs, arg)
			continue
		}
		var found []string
		err = filepath.WalkDir(arg, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				fmt.Fprintf(stderr, "gofastxml: %v\n", err)
				status = 1
				return nil
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".xml") {
				found = append(found, path)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "gofastxml: %v\n", err)
			status = 1
		}
		sort.Strings(found)
		inputs = append(inputs, found...)
	}

	out := stdout
	if *output != "" {
		file, err := os.Create(*output)
		if err != nil {
			fmt.Fprintf(stderr, "gofastxml: %v\n", err)
			return 1
		}
		defer file.Close()
		out = file
	}
	w := bufio.NewWriter(out)
	defer w.Flush()
	render := &gofastcore.RenderOptions{}
	renderFlags.Apply(render)

	written := 0
	for _, path := range inputs {
		if ctx.Err() != nil {
			fmt.Fprintln(stderr, "gofastxml: interrupted")
			return 1
		}
		o := opts
		o.Name = filepath.Base(path)
		doc, err := extractor.ExtractFile(ctx, path, &o)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", path, err)
			status = 1
			continue
		}
		if !*quiet {
			if note := remarks(doc); note != "" {
				fmt.Fprintf(stderr, "%s: %s\n", path, note)
			}
		}
		switch *format {
		case "json":
			enc := json.NewEncoder(w)
			enc.SetEscapeHTML(false)
			if len(inputs) == 1 {
				enc.SetIndent("", "  ")
			}
			err = enc.Encode(doc)
		case "markdown":
			if len(inputs) > 1 {
				if written > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "<!-- %s -->\n\n", strings.ReplaceAll(path, "--", "- -"))
			}
			if err = doc.WriteMarkdown(w, render); err == nil {
				_, err = fmt.Fprintln(w)
			}
		default:
			if len(inputs) > 1 {
				if written > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "==> %s <==\n", path)
			}
			if err = doc.WriteText(w, render); err == nil {
				_, err = fmt.Fprintln(w)
			}
		}
		if err == nil {
			err = w.Flush()
		}
		if err != nil {
			fmt.Fprintf(stderr, "gofastxml: writing %s: %v\n", path, err)
			return 1
		}
		written++
	}
	return status
}

// remarks sums up what a document's warnings say that a person running
// the command would want to know without reading the JSON: anything of
// severity error.
func remarks(doc *gofastcore.Document) string {
	counts := map[string]int{}
	for _, w := range doc.AllWarnings() {
		if w.Severity == gofastcore.SeverityError {
			counts[w.Code]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for i, code := range codes {
		if n := counts[code]; n > 1 {
			codes[i] = fmt.Sprintf("%s (%d)", code, n)
		}
	}
	return "warnings: " + strings.Join(codes, ", ")
}

// moduleVersion is the version this command was built from.
func moduleVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package words

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

var readTests = []struct {
	Name               string
	Input              string
	Output             [][]string
	UseFieldsPerRecord bool // false (default) means FieldsPerRecord is -1

	// These fields are copied into the Reader
	Comma            rune
	Comment          rune
	FieldsPerRecord  int
	LazyQuotes       bool
	TrimLeadingSpace bool
	AllowSingleQuote bool
	BackslashEscapes bool

	Error  string
	Line   int // Expected error line if != 0
	Column int // Expected error column if line != 0
}{
	{
		Name:   "Simple",
		Input:  "a,b,c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "CRLF",
		Input:  "a,b\r\nc,d\r\n",
		Output: [][]string{{"a", "b"}, {"c", "d"}},
	},
	{
		Name:   "BareCR",
		Input:  "a,b\rc,d\r\n",
		Output: [][]string{{"a", "b\rc", "d"}},
	},
	{
		Name:               "RFC4180test",
		UseFieldsPerRecord: true,
		Input: `#field1,field2,field3
"aaa","bb
b","ccc"
"a,a","b""bb","ccc"
zzz,yyy,xxx
`,
		Output: [][]string{
			{"#field1", "field2", "field3"},
			{"aaa", "bb\nb", "ccc"},
			{"a,a", `b"bb`, "ccc"},
			{"zzz", "yyy", "xxx"},
		},
	},
	{
		Name:   "NoEOLTest",
		Input:  "a,b,c",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "Semicolon",
		Comma:  ';',
		Input:  "a;b;c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name: "MultiLine",
		Input: `"two
line","one line","three
line
field"`,
		Output: [][]string{{"two\nline", "one line", "three\nline\nfield"}},
	},
	{
		Name:  "BlankLine",
		Input: "a,b,c\n\nd,e,f\n\n",
		Output: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	},
	{
		Name:             "TrimSpace",
		Input:            " a,  b,   c\n",
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "LeadingSpace",
		Input:  " a,  b,   c\n",
		Output: [][]string{{" a", "  b", "   c"}},
	},
	{
		Name:    "Comment",
		Comment: '#',
		Input:   "#1,2,3\na,b,c\n#comment",
		Output:  [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "NoComment",
		Input:  "#1,2,3\na,b,c",
		Output: [][]string{{"#1", "2", "3"}, {"a", "b", "c"}},
	},
	{
		Name:       "LazyQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a","b`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`, `b`}},
	},
	{
		Name:       "BareQuotes",
		LazyQuotes: true,
		Input:      `a "word","1"2",a"`,
		Output:     [][]string{{`a "word"`, `1"2`, `a"`}},
	},
	{
		Name:       "BareDoubleQuotes",
		LazyQuotes: true,
		Input:      `a""b,c`,
		Output:     [][]string{{`a""b`, `c`}},
	},
	{
		Name:  "BadDoubleQuotes",
		Input: `a""b,c`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 1,
	},
	{
		Name:             "TrimQuote",
		Input:            ` "a"," b",c`,
		TrimLeadingSpace: true,
		Output:           [][]string{{"a", " b", "c"}},
	},
	{
		Name:  "BadBareQuote",
		Input: `a "word","b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 2,
	},
	{
		Name:  "BadTrailingQuote",
		Input: `"a word",b"`,
		Error: `bare " in non-quoted-field`, Line: 1, Column: 10,
	},
	{
		Name:  "ExtraneousQuote",
		Input: `"a "word","b"`,
		Error: `extraneous " in field`, Line: 1, Column: 3,
	},
	{
		Name:               "BadFieldCount",
		UseFieldsPerRecord: true,
		Input:              "a,b,c\nd,e",
		Error:              "wrong number of fields", Line: 2,
	},
	{
		Name:               "BadFieldCount1",
		UseFieldsPerRecord: true,
		FieldsPerRecord:    2,
		Input:              `a,b,c`,
		Error:              "wrong number of fields", Line: 1,
	},
	{
		Name:   "FieldCount",
		Input:  "a,b,c\nd,e",
		Output: [][]string{{"a", "b", "c"}, {"d", "e"}},
	},
	{
		Name:   "TrailingCommaEOF",
		Input:  "a,b,c,",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:   "TrailingCommaEOL",
		Input:  "a,b,c,\n",
		Output: [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOF",
		TrimLeadingSpace: true,
		Input:            "a,b,c, ",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaSpaceEOL",
		TrimLeadingSpace: true,
		Input:            "a,b,c, \n",
		Output:           [][]string{{"a", "b", "c", ""}},
	},
	{
		Name:             "TrailingCommaLine3",
		TrimLeadingSpace: true,
		Input:            "a,b,c\nd,e,f\ng,hi,",
		Output:           [][]string{{"a", "b", "c"}, {"d", "e", "f"}, {"g", "hi", ""}},
	},
	{
		Name:   "NotTrailingComma3",
		Input:  "a,b,c, \n",
		Output: [][]string{{"a", "b", "c", " "}},
	},
	{
		Name: "CommaFieldTest",
		Input: `x,y,z,w
x,y,z,
x,y,,
x,,,
,,,
"x","y","z","w"
"x","y","z",""
"x","y","",""
"x","","",""
"","","",""
`,
		Output: [][]string{
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
			{"x", "y", "z", "w"},
			{"x", "y", "z", ""},
			{"x", "y", "", ""},
			{"x", "", "", ""},
			{"", "", "", ""},
		},
	},
	{
		Name:             "TrailingCommaIneffective1",
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	{
		Name:             "TrailingCommaIneffective2",
		TrimLeadingSpace: true,
		Input:            "a,b,\nc,d,e",
		Output: [][]string{
			{"a", "b", ""},
			{"c", "d", "e"},
		},
	},
	// Single-quote fields (opt-in via AllowSingleQuote).
	{
		Name:             "SingleQuote",
		AllowSingleQuote: true,
		Input:            "'abc'",
		Output:           [][]string{{"abc"}},
	},
	{
		Name:             "SingleQuoteSpace",
		AllowSingleQuote: true,
		Input:            "'a b'",
		Output:           [][]string{{"a b"}},
	},
	{
		Name:             "SingleQuoteDoubling",
		AllowSingleQuote: true,
		Input:            "'it''s'",
		Output:           [][]string{{"it's"}},
	},
	{
		// Without the flag, a single quote is an ordinary character.
		Name:   "SingleQuoteDefaultFalse",
		Input:  "'abc'",
		Output: [][]string{{"'abc'"}},
	},
	{
		// An apostrophe mid-field must never be treated as a quote, even when
		// AllowSingleQuote is enabled.
		Name:             "ApostropheInUnquoted",
		AllowSingleQuote: true,
		Input:            "don't",
		Output:           [][]string{{"don't"}},
	},
	{
		Name:             "SingleQuoteEscape",
		AllowSingleQuote: true,
		BackslashEscapes: true,
		Input:            `'a\'b'`,
		Output:           [][]string{{"a'b"}},
	},
	{
		Name:             "MixedQuotes",
		AllowSingleQuote: true,
		Input:            `"a",'b'`,
		Output:           [][]string{{"a", "b"}},
	},
	// Backslash escapes inside quoted fields (opt-in via BackslashEscapes).
	{
		Name:             "EscapedQuote",
		BackslashEscapes: true,
		Input:            `"a\"b"`,
		Output:           [][]string{{`a"b`}},
	},
	{
		Name:             "EscapedBackslash",
		BackslashEscapes: true,
		Input:            `"a\\b"`,
		Output:           [][]string{{`a\b`}},
	},
	{
		Name:             "EscapedNewline",
		BackslashEscapes: true,
		Input:            `"a\nb"`,
		Output:           [][]string{{"a\nb"}},
	},
	{
		Name:             "EscapedTab",
		BackslashEscapes: true,
		Input:            `"a\tb"`,
		Output:           [][]string{{"a\tb"}},
	},
	{
		Name:             "UnknownEscapeKept",
		BackslashEscapes: true,
		Input:            `"C:\Users"`,
		Output:           [][]string{{`C:\Users`}},
	},
	{
		Name:             "EscapeThenField",
		BackslashEscapes: true,
		Input:            `"a\"","b"`,
		Output:           [][]string{{`a"`, "b"}},
	},
	{
		// Without the flag, a backslash is ordinary and the bare quote errors.
		Name:  "EscapedQuoteDefaultFalse",
		Input: `"a\"b"`,
		Error: `extraneous " in field`,
	},
	{
		// When Comma is not whitespace, a tab is field content, not a delimiter.
		Name:   "TabIsContentInCSVMode",
		Input:  "a\tb,c\n",
		Output: [][]string{{"a\tb", "c"}},
	},
}

func TestRead(t *testing.T) {
	for _, tt := range readTests {
		r := NewReader(strings.NewReader(tt.Input))

		r.Comma = ',' // added config to match with old "csv" defaults - to maintain tests
		r.FieldsPerRecord = 0
		r.TrimLeadingSpace = false

		r.Comment = tt.Comment
		if tt.UseFieldsPerRecord {
			r.FieldsPerRecord = tt.FieldsPerRecord
		} else {
			r.FieldsPerRecord = -1
		}
		r.LazyQuotes = tt.LazyQuotes
		r.TrimLeadingSpace = tt.TrimLeadingSpace
		r.AllowSingleQuote = tt.AllowSingleQuote
		r.BackslashEscapes = tt.BackslashEscapes
		if tt.Comma != 0 {
			r.Comma = tt.Comma
		}
		out, err := r.ReadAll()
		perr, _ := err.(*ParseError)
		if tt.Error != "" {
			if err == nil || !strings.Contains(err.Error(), tt.Error) {
				t.Errorf("%s: error %v, want error %q", tt.Name, err, tt.Error)
			} else if tt.Line != 0 && (tt.Line != perr.Line || tt.Column != perr.Column) {
				t.Errorf("%s: error at %d:%d expected %d:%d", tt.Name, perr.Line, perr.Column, tt.Line, tt.Column)
			}
		} else if err != nil {
			t.Errorf("%s: unexpected error %v", tt.Name, err)
		} else if !reflect.DeepEqual(out, tt.Output) {
			t.Errorf("%s: out=%q want %q", tt.Name, out, tt.Output)
		}
	}
}

// readWordsTests exercises the whitespace-tokenizing behavior that NewReader
// configures by default. Unlike readTests above, these do not override Comma,
// so they cover the package's primary use case.
var readWordsTests = []struct {
	Name   string
	Input  string
	Output [][]string
}{
	{
		Name:   "Simple",
		Input:  "a b c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "MultipleSpacesCollapse",
		Input:  "a    b    c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		Name:   "LeadingSpaceTrimmed",
		Input:  "   a b\n",
		Output: [][]string{{"a", "b"}},
	},
	{
		Name:   "QuotedPhrase",
		Input:  "\"hello world\" foo\n",
		Output: [][]string{{"hello world", "foo"}},
	},
	{
		// FieldsPerRecord defaults to -1, so records may vary in length.
		Name:   "VariableFieldCounts",
		Input:  "a b\nc d e\n",
		Output: [][]string{{"a", "b"}, {"c", "d", "e"}},
	},
	{
		// Only the delimiter rune (space) separates fields; a tab is content.
		Name:   "TabIsDelimiter",
		Input:  "a\tb c\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		// Runs of tabs and spaces collapse into a single separator.
		Name:   "TabsAndSpacesCollapse",
		Input:  "a \t b\t\tc\n",
		Output: [][]string{{"a", "b", "c"}},
	},
	{
		// A tab inside a quoted field is preserved as content.
		Name:   "TabPreservedInQuotes",
		Input:  "\"a\tb\" c\n",
		Output: [][]string{{"a\tb", "c"}},
	},
}

func TestReadWords(t *testing.T) {
	for _, tt := range readWordsTests {
		// Use NewReader defaults directly, with no configuration overrides.
		r := NewReader(strings.NewReader(tt.Input))
		out, err := r.ReadAll()
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.Name, err)
			continue
		}
		if !reflect.DeepEqual(out, tt.Output) {
			t.Errorf("%s: out=%q want %q", tt.Name, out, tt.Output)
		}
	}
}

// TestParseErrorUnwrap verifies that ParseError exposes its inner sentinel error
// via Unwrap, so callers can use errors.Is and errors.As.
func TestParseErrorUnwrap(t *testing.T) {
	r := NewReader(strings.NewReader(`a"b`)) // bare quote in an unquoted field
	_, err := r.Read()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T (%v)", err, err)
	}
	if !errors.Is(err, ErrBareQuote) {
		t.Errorf("errors.Is(err, ErrBareQuote) = false, want true (err=%v)", err)
	}
	if !errors.Is(err, pe.Err) {
		t.Errorf("errors.Is(err, pe.Err) = false, want true (err=%v)", err)
	}
}

// readWordsQuotingTests exercises single-quote fields and backslash escapes in
// the package's default (space-delimited) context.
var readWordsQuotingTests = []struct {
	Name             string
	Input            string
	AllowSingleQuote bool
	BackslashEscapes bool
	Output           [][]string
}{
	{
		Name:             "SingleQuoteWord",
		AllowSingleQuote: true,
		Input:            "'hello world' foo\n",
		Output:           [][]string{{"hello world", "foo"}},
	},
	{
		Name:             "SingleQuoteDoubling",
		AllowSingleQuote: true,
		Input:            "'it''s' bar\n",
		Output:           [][]string{{"it's", "bar"}},
	},
	{
		Name:             "EscapedQuote",
		BackslashEscapes: true,
		Input:            `"a\"b" c` + "\n",
		Output:           [][]string{{`a"b`, "c"}},
	},
	{
		Name:             "EscapedNewline",
		BackslashEscapes: true,
		Input:            `"a\nb" c` + "\n",
		Output:           [][]string{{"a\nb", "c"}},
	},
	{
		Name:             "BothFeatures",
		AllowSingleQuote: true,
		BackslashEscapes: true,
		Input:            `'a\'b' "c\"d"` + "\n",
		Output:           [][]string{{"a'b", `c"d`}},
	},
}

func TestReadWordsQuoting(t *testing.T) {
	for _, tt := range readWordsQuotingTests {
		r := NewReader(strings.NewReader(tt.Input))
		r.AllowSingleQuote = tt.AllowSingleQuote
		r.BackslashEscapes = tt.BackslashEscapes
		out, err := r.ReadAll()
		if err != nil {
			t.Errorf("%s: unexpected error %v", tt.Name, err)
			continue
		}
		if !reflect.DeepEqual(out, tt.Output) {
			t.Errorf("%s: out=%#v want %#v", tt.Name, out, tt.Output)
		}
	}
}

// TestDecodeEscape covers the pure escape-translation helper directly.
func TestDecodeEscape(t *testing.T) {
	tests := []struct {
		in   rune
		want string
	}{
		{'n', "\n"},
		{'t', "\t"},
		{'r', "\r"},
		{'"', `"`},
		{'\'', `'`},
		{'\\', `\`},
		{'U', `\U`}, // unknown escape: kept verbatim
		{' ', `\ `}, // backslash before a space: kept verbatim
	}
	for _, tt := range tests {
		if got := decodeEscape(tt.in); got != tt.want {
			t.Errorf("decodeEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

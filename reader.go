// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package words reads and writes blank-separated (whitespace-delimited) values.
//
// It is derived from the standard library's encoding/csv and retuned so that the
// default behavior is splitting records on whitespace (space and tab) — intended
// as a rudimentary lexer for the front end of an interpreter. The configurable Reader
// and Writer types also support an arbitrary single-rune delimiter, quoted fields,
// and comments, so the package can fall back to ordinary comma- or
// semicolon-separated behavior when needed.
//
// # Input format
//
// A file contains zero or more records of one or more fields per record. Each
// record is separated by the newline character. The final record may optionally
// be followed by a trailing newline.
//
// By default the field delimiter is a space and leading whitespace is trimmed.
// Because the delimiter is itself whitespace, every other whitespace rune (most
// usefully the tab) separates fields just as a space does, so runs of spaces and
// tabs split records into fields:
//
//	field1	field2	field3
//
// results in the fields
//
//	{`field1`, `field2`, `field3`}
//
// (Set Comma to a non-whitespace rune such as ',' to switch to ordinary
// single-delimiter behavior, in which a tab becomes field content.) Whitespace
// inside a field is preserved by quoting the field.
//
// Carriage returns before a newline are silently folded to a single newline.
//
// A truly empty line (containing only the newline) is ignored. A line that
// contains only whitespace, however, is not blank in this sense: it yields a
// record with a single empty field ([""]).
//
// Fields which start and end with the quote character " are called quoted fields.
// The opening and closing quotes are not part of the field.
//
//	normal-string "quoted-field"
//
// results in the fields
//
//	{`normal-string`, `quoted-field`}
//
// Within a quoted field, a quote character immediately followed by a second quote
// is treated as a single literal quote.
//
//	"the ""word"" is true" "a ""quoted-field"""
//
// results in
//
//	{`the "word" is true`, `a "quoted-field"`}
//
// Newlines and the delimiter character may be included inside a quoted field:
//
//	"multi-line
//	field" "space is  "
//
// results in
//
//	{`multi-line
//	field`, `space is  `}
//
// # Quoting and escapes
//
// By default only the double quote opens a quoted field. Set AllowSingleQuote to
// also accept single-quoted fields, which follow the same rules, including
// doubling (two adjacent single quotes stand for one literal single quote):
//
//	'single "quoted" field' 'it''s'
//
// results in
//
//	{`single "quoted" field`, `it's`}
//
// Set BackslashEscapes to recognize C-style backslash escapes inside quoted
// fields. The recognized sequences are \" \\ \n \t \r and \', mapping to a quote,
// a backslash, a newline, a tab, a carriage return, and a single quote; any other
// \X is kept verbatim, both characters, so input that was not meant as an escape
// is preserved. Escapes apply only inside quoted fields. With BackslashEscapes
// enabled:
//
//	"a\"b\nc\td"
//
// yields a single field whose value is the rune sequence a, a quote, b, a newline,
// c, a tab, d.
//
// # Errors
//
// Parse errors carry a 1-based line number and 0-based column (rune index). The
// underlying error is reachable via errors.Is and errors.As thanks to ParseError's
// Unwrap method, so callers can test for the sentinel errors declared in this
// package:
//
//	r := words.NewReader(strings.NewReader(input))
//	if _, err := r.Read(); err != nil {
//	    if errors.Is(err, words.ErrFieldCount) {
//	        // ...handle a field-count mismatch...
//	    }
//	}
package words

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
)

// A ParseError is returned for parsing errors.
// Line numbers are 1-based; columns are 0-based rune indices.
type ParseError struct {
	Line   int   // Line where the error occurred
	Column int   // Column (rune index) where the error occurred
	Err    error // The actual error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Err)
}

// Unwrap returns the underlying error, allowing errors.Is and errors.As to match
// the sentinel errors declared in this package (for example ErrFieldCount).
func (e *ParseError) Unwrap() error {
	return e.Err
}

// These are the errors that can be returned as the inner error of a ParseError.
var (
	ErrBareQuote  = errors.New("bare \" in non-quoted-field")
	ErrQuote      = errors.New("extraneous \" in field")
	ErrFieldCount = errors.New("wrong number of fields in line")
)

// A Reader reads records from a blank-separated (or delimited) file.
//
// As returned by NewReader, a Reader splits records on whitespace: the field
// delimiter is a space (and, because it is whitespace, tabs separate fields the
// same way), leading whitespace is trimmed, and records may contain a variable
// number of fields. The exported fields can be changed before the first call to
// Read or ReadAll to customize the details.
//
//   - Comma is the field delimiter. It defaults to ' ' (space). When Comma is
//     whitespace, every whitespace rune separates fields; otherwise only Comma
//     does and other whitespace is field content.
//   - Comment, if non-zero, is the comment character. Lines beginning with it are
//     ignored. It defaults to 0 (no comments).
//   - FieldsPerRecord governs field-count validation. If positive, Read requires
//     each record to have exactly that many fields. If 0, Read sets it to the
//     number of fields in the first record, so subsequent records must match. If
//     negative (the default), no check is made and records may vary in length.
//   - LazyQuotes, if true, allows a quote to appear in an unquoted field and a
//     non-doubled quote to appear in a quoted field.
//   - TrimLeadingSpace, if true (the default), ignores leading whitespace in a
//     field.
//   - AllowSingleQuote, if true, also treats a field wrapped in single quotes as
//     a quoted field. The same doubling rule as for double quotes applies: a pair
//     of adjacent single quotes stands for one literal single quote. It defaults
//     to false, so apostrophes inside otherwise-unquoted text such as "don't" are
//     left untouched.
//   - BackslashEscapes, if true, recognizes backslash escape sequences inside
//     quoted fields: \" \\ \n \t \r \' map to their usual values, and any other
//     \X is kept literally (backslash and X), so data such as "C:\Users" is never
//     corrupted. It defaults to false. Escapes are recognized only inside quoted
//     fields.
type Reader struct {
	Comma            rune // field delimiter (set to ' ' by NewReader)
	Comment          rune // comment character for start of line, or 0
	FieldsPerRecord  int  // expected fields per record (see type doc)
	LazyQuotes       bool // allow lazy quotes
	TrimLeadingSpace bool // trim leading space
	AllowSingleQuote bool // also treat '...' as a quoted field
	BackslashEscapes bool // recognize \-escapes inside quoted fields
	line             int
	column           int
	r                *bufio.Reader
	field            bytes.Buffer
}

// NewReader returns a new Reader that reads from r, configured for whitespace
// tokenization: Comma is set to ' ', TrimLeadingSpace to true, and
// FieldsPerRecord to -1 (variable field counts allowed).
func NewReader(r io.Reader) *Reader {
	return &Reader{
		Comma:            ' ',
		FieldsPerRecord:  -1,
		TrimLeadingSpace: true,
		r:                bufio.NewReader(r),
	}
}

// error creates a new ParseError based on err.
func (r *Reader) error(err error) error {
	return &ParseError{
		Line:   r.line,
		Column: r.column,
		Err:    err,
	}
}

// Read reads one record from r. The record is a slice of strings with each
// string representing one field.
func (r *Reader) Read() (record []string, err error) {
	for {
		record, err = r.parseRecord()
		if record != nil {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	if r.FieldsPerRecord > 0 {
		if len(record) != r.FieldsPerRecord {
			r.column = 0 // report at start of record
			return record, r.error(ErrFieldCount)
		}
	} else if r.FieldsPerRecord == 0 {
		r.FieldsPerRecord = len(record)
	}
	return record, nil
}

// ReadAll reads all the remaining records from r.
// Each record is a slice of fields.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read until EOF, it does not treat end of file as an error to be
// reported.
func (r *Reader) ReadAll() (records [][]string, err error) {
	for {
		record, err := r.Read()
		if err == io.EOF {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
}

// readRune reads one rune from r, folding \r\n to \n and keeping track
// of how far into the line we have read. r.column will point to the start
// of this rune, not the end of this rune.
func (r *Reader) readRune() (rune, error) {
	r1, _, err := r.r.ReadRune()

	// Handle \r\n here. We make the simplifying assumption that
	// anytime \r is followed by \n that it can be folded to \n.
	// We will not detect files which contain both \r\n and bare \n.
	if r1 == '\r' {
		r1, _, err = r.r.ReadRune()
		if err == nil {
			if r1 != '\n' {
				r.r.UnreadRune()
				r1 = '\r'
			}
		}
	}
	r.column++
	return r1, err
}

// unreadRune puts the last rune read from r back.
func (r *Reader) unreadRune() {
	r.r.UnreadRune()
	r.column--
}

// skip reads runes up to and including the rune delim or until error.
func (r *Reader) skip(delim rune) error {
	for {
		r1, err := r.readRune()
		if err != nil {
			return err
		}
		if r1 == delim {
			return nil
		}
	}
}

// parseRecord reads and parses a single record from r.
func (r *Reader) parseRecord() (fields []string, err error) {
	// Each record starts on a new line. We increment our line
	// number (lines start at 1, not 0) and set column to -1
	// so as we increment in readRune it points to the character we read.
	r.line++
	r.column = -1

	// Peek at the first rune. If it is an error we are done.
	// If we support comments and it is the comment character,
	// then skip to the end of line.
	r1, _, err := r.r.ReadRune()
	if err != nil {
		return nil, err
	}

	if r.Comment != 0 && r1 == r.Comment {
		return nil, r.skip('\n')
	}
	r.r.UnreadRune()

	// At this point we have at least one field.
	for firstField := true; ; firstField = false {
		haveField, delim, err := r.parseField(firstField)
		if haveField {
			fields = append(fields, r.field.String())
		}
		if delim == '\n' || err == io.EOF {
			return fields, err
		} else if err != nil {
			return nil, err
		}
	}
}

// parseField parses the next field in the record. The read field is
// located in r.field. Delim is the first character not part of the field
// (r.Comma or '\n'). startOfRecord reports whether this is the first field of
// the record; it is used to tell a whitespace-only line (which yields a single
// empty field) apart from trailing whitespace after a field (which yields none).
func (r *Reader) parseField(startOfRecord bool) (haveField bool, delim rune, err error) {
	r.field.Reset()

	r1, err := r.readRune()
	for err == nil && r.TrimLeadingSpace && r1 != '\n' && unicode.IsSpace(r1) {
		r1, err = r.readRune()
	}

	// We ran to the end of input while skipping leading whitespace, so the
	// field buffer is empty. When Comma is not whitespace (CSV mode) that empty
	// field is genuine — a preceding delimiter created it — so it is reported.
	// When Comma is itself whitespace, a trailing run of whitespace is just a
	// collapsed separator and produces no field, except at the start of a
	// record, where a whitespace-only line yields a single empty field.
	if err == io.EOF {
		if r.column != 0 && (!unicode.IsSpace(r.Comma) || startOfRecord) {
			return true, 0, err
		}
		return false, 0, err
	}
	if err != nil {
		return false, 0, err
	}

	// A field wrapped in a quote character is parsed by parseQuotedField.
	if q, ok := r.quoteChar(r1); ok {
		return r.parseQuotedField(q)
	}

	// A leading delimiter produces an empty field.
	if r.isDelim(r1) {
		return true, r1, nil
	}
	// A blank line, a whitespace-only line, or trailing whitespace before a
	// newline.
	if r1 == '\n' {
		// A truly blank line (only the newline) yields no field.
		if r.column == 0 {
			return false, r1, nil
		}
		// As above: CSV mode keeps the empty field; whitespace mode keeps it
		// only for a whitespace-only line at the start of the record.
		if !unicode.IsSpace(r.Comma) || startOfRecord {
			return true, r1, nil
		}
		return false, r1, nil
	}

	// unquoted field
	for {
		r.field.WriteRune(r1)
		r1, err = r.readRune()
		if err != nil || r.isDelim(r1) {
			break
		}
		if r1 == '\n' {
			return true, r1, nil
		}
		if !r.LazyQuotes && r1 == '"' {
			return false, 0, r.error(ErrBareQuote)
		}
	}

	if err != nil {
		if err == io.EOF {
			return true, 0, err
		}
		return false, 0, err
	}

	return true, r1, nil
}

// isDelim reports whether r1 terminates a field. The rune r.Comma always
// terminates a field. In addition, when Comma is itself a whitespace rune (the
// default is a space), any other whitespace rune also terminates a field — so
// tabs separate fields exactly the same way spaces do — except for the newline,
// which terminates a record rather than a field. When Comma is not whitespace
// (for example ','), only r.Comma terminates a field and other whitespace is
// field content.
func (r *Reader) isDelim(r1 rune) bool {
	if r1 == r.Comma || r1 == '\n' {
		return r1 == r.Comma
	}
	return unicode.IsSpace(r.Comma) && unicode.IsSpace(r1)
}

// quoteChar reports whether r1 opens a quoted field, and if so returns the quote
// rune. The double quote always opens a quote; the single quote opens one only
// when AllowSingleQuote is set, so by default an apostrophe such as the one in
// don't is an ordinary character.
func (r *Reader) quoteChar(r1 rune) (quote rune, ok bool) {
	if r1 == '"' {
		return '"', true
	}
	if r.AllowSingleQuote && r1 == '\'' {
		return '\'', true
	}
	return 0, false
}

// parseQuotedField reads the body of a field opened by quote and returns when it
// is terminated by the delimiter, a newline, or EOF. The parsed content is left
// in r.field. The return values match parseField: haveField is true on any
// non-error return, delim is the terminating rune (r.Comma or '\n'; 0 on EOF),
// and err is io.EOF when the field runs to the end of input.
func (r *Reader) parseQuotedField(quote rune) (haveField bool, delim rune, err error) {
	for {
		r1, err := r.readRune()
		if err != nil {
			if err == io.EOF {
				if r.LazyQuotes {
					return true, 0, err
				}
				return false, 0, r.error(ErrQuote)
			}
			return false, 0, err
		}

		// Backslash escapes are recognized only inside quoted fields.
		if r.BackslashEscapes && r1 == '\\' {
			esc, eerr := r.readRune()
			if eerr != nil {
				// Unterminated escape: a backslash at EOF or a read error.
				// Keep the backslash, then apply the usual end-of-input rules.
				r.field.WriteRune('\\')
				if eerr == io.EOF {
					if r.LazyQuotes {
						return true, 0, eerr
					}
					return false, 0, r.error(ErrQuote)
				}
				return false, 0, eerr
			}
			r.field.WriteString(decodeEscape(esc))
			continue
		}

		switch r1 {
		case quote:
			// Possible end of field, or a doubled quote (a literal quote).
			next, nerr := r.readRune()
			if nerr != nil {
				if nerr == io.EOF {
					return true, 0, nerr
				}
				return false, 0, nerr
			}
			if r.isDelim(next) || next == '\n' {
				return true, next, nil
			}
			if next != quote {
				if !r.LazyQuotes {
					r.column--
					return false, 0, r.error(ErrQuote)
				}
				// Lazy: accept the bare quote and continue with next.
				r.field.WriteRune(quote)
			}
			r1 = next
		case '\n':
			r.line++
			r.column = -1
		}
		r.field.WriteRune(r1)
	}
}

// decodeEscape translates the rune following a backslash inside a quoted field
// when BackslashEscapes is enabled. Recognized sequences map to their usual
// values; any unrecognized sequence is returned verbatim, backslash included, so
// data such as "C:\Users" is never altered.
func decodeEscape(c rune) string {
	switch c {
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case 'r':
		return "\r"
	case '"', '\'', '\\':
		return string(c)
	}
	return "\\" + string(c)
}

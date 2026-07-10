// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package words

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

// writeTests exercises the quoting and line-termination logic. Most cases are
// independent of the delimiter; the two that embed a comma set Comma explicitly
// to ',' (the Writer default is now a space — see writeWordsTests).
var writeTests = []struct {
	Input   [][]string
	Output  string
	UseCRLF bool
	Comma   rune
}{
	{Input: [][]string{{"abc"}}, Output: "abc\n"},
	{Input: [][]string{{"abc"}}, Output: "abc\r\n", UseCRLF: true},
	{Input: [][]string{{`"abc"`}}, Output: `"""abc"""` + "\n"},
	{Input: [][]string{{`a"b`}}, Output: `"a""b"` + "\n"},
	{Input: [][]string{{`"a"b"`}}, Output: `"""a""b"""` + "\n"},
	{Input: [][]string{{" abc"}}, Output: `" abc"` + "\n"},
	{Input: [][]string{{"abc,def"}}, Output: `"abc,def"` + "\n", Comma: ','},
	{Input: [][]string{{"abc", "def"}}, Output: "abc,def\n", Comma: ','},
	{Input: [][]string{{"abc"}, {"def"}}, Output: "abc\ndef\n"},
	{Input: [][]string{{"abc\ndef"}}, Output: "\"abc\ndef\"\n"},
	{Input: [][]string{{"abc\ndef"}}, Output: "\"abc\r\ndef\"\r\n", UseCRLF: true},
}

func TestWrite(t *testing.T) {
	for n, tt := range writeTests {
		b := &bytes.Buffer{}
		f := NewWriter(b)
		f.UseCRLF = tt.UseCRLF
		if tt.Comma != 0 {
			f.Comma = tt.Comma
		}
		err := f.WriteAll(tt.Input)
		if err != nil {
			t.Errorf("Unexpected error: %s\n", err)
		}
		out := b.String()
		if out != tt.Output {
			t.Errorf("#%d: out=%q want %q", n, out, tt.Output)
		}
	}
}

// writeWordsTests exercises the default space delimiter that NewWriter sets.
var writeWordsTests = []struct {
	Name    string
	Input   [][]string
	Output  string
	UseCRLF bool
}{
	{Name: "Single", Input: [][]string{{"abc"}}, Output: "abc\n"},
	{Name: "TwoFields", Input: [][]string{{"abc", "def"}}, Output: "abc def\n"},
	{Name: "EmbeddedSpaceQuoted", Input: [][]string{{"hello world"}}, Output: "\"hello world\"\n"},
	{Name: "LeadingSpaceQuoted", Input: [][]string{{" abc"}}, Output: `" abc"` + "\n"},
	{Name: "EmptyFieldQuoted", Input: [][]string{{""}}, Output: `""` + "\n"},
	{Name: "CRLF", Input: [][]string{{"abc", "def"}}, Output: "abc def\r\n", UseCRLF: true},
}

func TestWriteWords(t *testing.T) {
	for _, tt := range writeWordsTests {
		b := &bytes.Buffer{}
		w := NewWriter(b)
		w.UseCRLF = tt.UseCRLF
		if err := w.WriteAll(tt.Input); err != nil {
			t.Errorf("%s: unexpected error: %s", tt.Name, err)
			continue
		}
		if got := b.String(); got != tt.Output {
			t.Errorf("%s: out=%q want %q", tt.Name, got, tt.Output)
		}
	}
}

// TestReadWriteRoundTrip verifies that a record written by Writer (with its
// space default) can be read back by Reader (with its space default) unchanged,
// including fields with embedded spaces and empty fields.
func TestReadWriteRoundTrip(t *testing.T) {
	records := [][]string{
		{"set", "x", "=", "hello world"},
		{"print", "x", "", "y"},
		{"plain"},
	}

	var b bytes.Buffer
	w := NewWriter(&b)
	if err := w.WriteAll(records); err != nil {
		t.Fatalf("WriteAll: %s", err)
	}

	r := NewReader(&b)
	out, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %s", err)
	}
	if !reflect.DeepEqual(out, records) {
		t.Errorf("round-trip mismatch:\n got  %#v\n want %#v", out, records)
	}
}

type errorWriter struct{}

func (e errorWriter) Write(b []byte) (int, error) {
	return 0, errors.New("Test")
}

func TestError(t *testing.T) {
	b := &bytes.Buffer{}
	f := NewWriter(b)
	f.Write([]string{"abc"})
	f.Flush()
	err := f.Error()

	if err != nil {
		t.Errorf("Unexpected error: %s\n", err)
	}

	f = NewWriter(errorWriter{})
	f.Write([]string{"abc"})
	f.Flush()
	err = f.Error()

	if err == nil {
		t.Error("Error should not be nil")
	}
}

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package words_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/pschlump/gowords"
)

// ExampleReader demonstrates the default behavior: the input is tokenized on
// runs of whitespace, while a double-quoted phrase is kept as a single field
// with its internal spaces preserved.
func ExampleReader() {
	in := `set x = "hello world"
print x y z`
	r := words.NewReader(strings.NewReader(in))
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%q\n", rec)
	}
	// Output:
	// ["set" "x" "=" "hello world"]
	// ["print" "x" "y" "z"]
}

// ExampleReader_comment shows the Comment option: lines that begin with the
// comment character are skipped entirely.
func ExampleReader_comment() {
	in := `# this line is skipped
print hello
# and so is this one`
	r := words.NewReader(strings.NewReader(in))
	r.Comment = '#'
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%q\n", records)
	// Output:
	// [["print" "hello"]]
}

// ExampleReader_fieldsPerRecord shows how to require a fixed number of fields
// per record: FieldsPerRecord is set after NewReader, and a mismatch is reported
// as a ParseError wrapping ErrFieldCount.
func ExampleReader_fieldsPerRecord() {
	in := "a b c\nd e" // second line has only two fields
	r := words.NewReader(strings.NewReader(in))
	r.FieldsPerRecord = 3
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		fmt.Printf("%q\n", rec)
	}
	// Output:
	// ["a" "b" "c"]
	// error: line 2, column 0: wrong number of fields in line
}

// ExampleReadAll reads an entire input into a slice of records in one call.
func ExampleReadAll() {
	in := `alpha beta gamma
one two`
	r := words.NewReader(strings.NewReader(in))
	records, err := r.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%q\n", records)
	// Output:
	// [["alpha" "beta" "gamma"] ["one" "two"]]
}

// ExampleWriter writes records using the default space delimiter. A field that
// contains the delimiter (here, an embedded space) is automatically quoted.
func ExampleWriter() {
	var b bytes.Buffer
	w := words.NewWriter(&b)
	records := [][]string{
		{"set", "x", "=", "hello world"},
		{"print", "x"},
	}
	if err := w.WriteAll(records); err != nil {
		log.Fatal(err)
	}
	fmt.Print(b.String())
	// Output:
	// set x = "hello world"
	// print x
}

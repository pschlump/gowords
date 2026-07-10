# gowords

[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/pschlump/Go-FTL/master/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/pschlump/gowords.svg)](https://pkg.go.dev/github.com/pschlump/gowords)

`gowords` (package `words`) is a fork of the standard library's
[`encoding/csv`](https://pkg.go.dev/encoding/csv), retuned so that the default is
to read and write **whitespace-delimited words**. It is intended as a small,
robust lexer for the front end of an interpreter: each line becomes a record, and
runs of spaces split each record into fields.

```go
import "github.com/pschlump/gowords"
```

## Why not `encoding/csv`?

`encoding/csv` defaults to comma-separated values. `gowords` defaults to a single
**space** as the delimiter, trims leading whitespace, and allows a variable
number of fields per record — so tokenizing free-form, command-style text works
out of the box:

```go
r := words.NewReader(strings.NewReader(`set x = "hello world"`))
rec, _ := r.Read()
// rec == []string{"set", "x", "=", "hello world"}
```

A field wrapped in double quotes keeps its internal whitespace; quotes inside a
quoted field are doubled, as in CSV.

## Reading

```go
r := words.NewReader(stdin)
for {
    record, err := r.Read()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(record)
}
// ...or read everything at once:
all, err := words.NewReader(stdin).ReadAll()
```

### Reader configuration

Tune a `Reader` by changing its exported fields before the first call to `Read`
or `ReadAll`:

| Field              | Default | Meaning                                                                   |
| ------------------ | ------- | ------------------------------------------------------------------------- |
| `Comma`            | `' '`   | Field delimiter. Set to `','` for ordinary CSV.                           |
| `Comment`          | `0`     | If non-zero, lines beginning with this rune are skipped.                  |
| `FieldsPerRecord`  | `-1`    | If `>0`, require exactly that many fields; `0` locks to the first record. |
| `LazyQuotes`       | `false` | Tolerate quotes in unquoted fields and non-doubled quotes.                |
| `TrimLeadingSpace` | `true`  | Trim whitespace before each field.                                        |

> **Note:** only the `Comma` rune separates fields. Other whitespace (e.g. a tab)
> between two non-space characters is preserved as field content, though
> whitespace at the *start* of a field is trimmed. A truly empty line is ignored;
> a line containing only whitespace yields a single empty-field record (`[""]`).

Parse errors are `*words.ParseError` and carry a 1-based line and 0-based column.
The underlying sentinel (`words.ErrFieldCount`, `words.ErrBareQuote`,
`words.ErrQuote`) is reachable through `errors.Is` / `errors.As`.

## Writing

`Writer` is the inverse of `Reader` and shares the same space default, so a
read→write round-trip stays whitespace-delimited. Fields are auto-quoted when they
are empty or contain the delimiter, a quote, a newline, or leading whitespace.

```go
var b bytes.Buffer
w := words.NewWriter(&b)
w.WriteAll([][]string{
    {"set", "x", "=", "hello world"},
    {"print", "x"},
})
// b.String() ==
//   set x = "hello world"
//   print x
```

Set `w.Comma = ','` and `w.UseCRLF = true` to emit RFC 4180-style CSV instead.
Call `w.Flush()` (or use `WriteAll`, which flushes for you) and then `w.Error()`
to surface any write error.

## License

Derived from the Go Authors' `encoding/csv`, distributed under the same
BSD-style license found in the [LICENSE](LICENSE) file.

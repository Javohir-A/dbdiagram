// Package dbml parses the practical subset of DBML that the app's editor
// uses and turns it into a schema model, a short text summary and SQL.
//
// Supported: Table (schema-qualified names, aliases, headerColor, notes,
// indexes blocks), columns with settings (pk, not null, null, unique,
// increment, default, note, inline ref), Enum, Ref in short and long form
// including composite endpoints (`t.(a, b)`), TableGroup, comments and all
// string forms.
//
// Not supported: Project settings (the block is skipped), sticky Notes
// (skipped), ref settings such as on delete (parsed and dropped),
// endpoints with more than schema.table.column, many-to-many junction
// tables in SQL, SQL import and the Oracle/MSSQL dialects.
package dbml

import (
	"fmt"
	"sort"
	"strings"
)

// maxErrors bounds the error list so a hostile document cannot make the
// result huge; the first errors are the useful ones anyway.
const maxErrors = 100

// Error is one problem found in the source. Line and Column are 1-based and
// Column counts characters, not bytes.
type Error struct {
	Line    int
	Column  int
	Message string
}

func (e Error) Error() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message) }

type pos struct{ line, col int }

// DefaultKind says how a column default has to be written back out.
type DefaultKind string

const (
	DefaultString DefaultKind = "string" // 'text' or "text"
	DefaultNumber DefaultKind = "number" // 12, -1.5, true, false, null
	DefaultExpr   DefaultKind = "expr"   // `now()`
)

// Schema is everything one document declares, in source order. Table names in
// Refs and Groups are resolved to Table.ID() (aliases are replaced).
type Schema struct {
	Tables []Table
	Refs   []Ref
	Enums  []Enum
	Groups []Group
}

type Table struct {
	Schema  string // "public" when the source gave none
	Name    string
	Alias   string
	Note    string
	Color   string // headerColor, e.g. "#3498db"
	Columns []Column
	Indexes []Index
	at      pos
}

// ID is the name the app uses for the table: the bare name in the public
// schema, otherwise schema.name.
func (t *Table) ID() string { return idOf(t.Schema, t.Name) }

func idOf(schema, name string) string {
	if schema == "" || schema == "public" {
		return name
	}

	return schema + "." + name
}

type Column struct {
	Name        string
	Type        string // as written, including arguments: varchar(255), int[]
	PK          bool
	NotNull     bool
	Unique      bool
	Increment   bool
	Default     *string
	DefaultKind DefaultKind
	Note        string
	at          pos
}

type IndexColumn struct {
	Name string
	Expr bool // Name is a backtick expression, not a column
}

type Index struct {
	Name    string
	Columns []IndexColumn
	Unique  bool
	PK      bool
	Type    string
	Note    string
	at      pos
}

// Ref is a relationship. Relation is one of "<", ">", "-", "<>" and reads
// left to right: `a.x > b.y` is many-to-one from a to b.
type Ref struct {
	Name     string
	From, To Endpoint
	Relation string
	at       pos
}

type Endpoint struct {
	Table   string
	Columns []string
	at      pos
}

type Enum struct {
	Schema string
	Name   string
	Values []EnumValue
	at     pos
}

func (e *Enum) ID() string { return idOf(e.Schema, e.Name) }

type EnumValue struct {
	Name string
	Note string
}

type Group struct {
	Name   string
	Tables []string
	at     []pos
	pos    pos
}

type errList struct {
	list []Error
	seen map[Error]bool
}

func (l *errList) add(line, col int, format string, args ...any) {
	e := Error{Line: line, Column: col, Message: fmt.Sprintf(format, args...)}
	if len(l.list) >= maxErrors || l.seen[e] {
		return
	}
	if l.seen == nil {
		l.seen = map[Error]bool{}
	}
	l.seen[e] = true
	l.list = append(l.list, e)
}

func (l *errList) sorted() []Error {
	sort.SliceStable(l.list, func(i, j int) bool {
		a, b := l.list[i], l.list[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}

		return a.Column < b.Column
	})

	return l.list
}

// Parse reads a document. The schema is always non-nil and holds whatever
// could be understood; the errors are every problem found, in source order. It
// never panics, whatever the input.
func Parse(src string) (*Schema, []Error) {
	var (
		errs = &errList{}
		p    = &parser{errs: errs, s: &Schema{}}
	)

	p.toks = lex(src, errs)
	p.file()
	p.check()

	return p.s, errs.sorted()
}

func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ", "\v", " ", "\f", " ", " ", " ", " ", " ", "\u0085", " ").Replace(s)
}

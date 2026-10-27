package dbml_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/pkg/dbml"
)

// sample is the document the app creates for a new diagram (frontend
// src/lib/dbml.ts SAMPLE_DBML); the frontend shows 2 tables, 10 columns, 1 ref.
const sample = "Table users {\n  id integer [pk, increment]\n  username varchar [not null, unique]\n  email varchar [not null]\n  created_at timestamp [default: `now()`]\n}\n\n" +
	"Table posts {\n  id integer [pk, increment]\n  title varchar [not null]\n  body text [note: 'Content of the post']\n  user_id integer [not null]\n  status post_status\n  created_at timestamp\n}\n\n" +
	"Enum post_status {\n  draft\n  published\n}\n\nRef: posts.user_id > users.id // many-to-one\n"

func mustParse(t *testing.T, src string) *dbml.Schema {
	t.Helper()

	s, errs := dbml.Parse(src)
	require.Empty(t, errs)

	return s
}

func positions(errs []dbml.Error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, fmt.Sprintf("%d:%d", e.Line, e.Column))
	}

	return out
}

func TestSampleMatchesTheFrontend(t *testing.T) {
	s := mustParse(t, sample)

	require.Len(t, s.Tables, 2)
	require.Len(t, s.Tables[0].Columns, 4)
	require.Len(t, s.Tables[1].Columns, 6)
	require.Len(t, s.Refs, 1)
	require.Len(t, s.Enums, 1)

	id := s.Tables[0].Columns[0]
	require.True(t, id.PK)
	require.True(t, id.Increment)

	created := s.Tables[0].Columns[3]
	require.Equal(t, "now()", *created.Default)
	require.Equal(t, dbml.DefaultExpr, created.DefaultKind)
	require.Equal(t, "Content of the post", s.Tables[1].Columns[2].Note)

	r := s.Refs[0]
	require.Equal(t, ">", r.Relation)
	require.Equal(t, "posts", r.From.Table)
	require.Equal(t, []string{"user_id"}, r.From.Columns)
	require.Equal(t, "users", r.To.Table)
}

func TestSummary(t *testing.T) {
	want := "users(id integer pk increment, username varchar not null unique, email varchar not null, created_at timestamp default `now()`)\n" +
		"posts(id integer pk increment, title varchar not null, body text, user_id integer not null, status post_status, created_at timestamp)\n\n" +
		"relationships:\nposts.user_id > users.id\n\nenums:\npost_status: draft, published"

	require.Equal(t, want, mustParse(t, sample).Summary())
	require.Equal(t, "", mustParse(t, "").Summary())
}

func TestParseConstructs(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		check func(t *testing.T, s *dbml.Schema)
	}{
		{"schema qualified table and alias", "Table auth.users as U { id int }\nTable public.logs { id int }\nRef: U.id - logs.id\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "auth", s.Tables[0].Schema)
			require.Equal(t, "auth.users", s.Tables[0].ID())
			require.Equal(t, "U", s.Tables[0].Alias)
			require.Equal(t, "logs", s.Tables[1].ID(), "public is dropped, like the frontend")
			require.Equal(t, "auth.users", s.Refs[0].From.Table, "the alias is resolved")
		}},
		{"header color and notes", "Table t [headerColor: #3498db, note: 'x'] {\n id int\n Note: 'table note'\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "#3498db", s.Tables[0].Color)
			require.Equal(t, "table note", s.Tables[0].Note)
		}},
		{"note block and triple quotes", "Table t {\n id int\n Note {\n '''multi\nline'''\n }\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "multi\nline", s.Tables[0].Note)
		}},
		{"types with arguments and arrays", "Table t {\n a varchar(255)\n b decimal(10,2)\n c \"timestamp with time zone\"\n d int[]\n e int[3] [not null]\n f enum('a','b\\'c')\n}\n", func(t *testing.T, s *dbml.Schema) {
			var got []string
			for _, c := range s.Tables[0].Columns {
				got = append(got, c.Type)
			}
			require.Equal(t, []string{"varchar(255)", "decimal(10,2)", "timestamp with time zone", "int[]", "int[3]", "enum('a','b''c')"}, got)
			require.True(t, s.Tables[0].Columns[4].NotNull)
		}},
		{"all column settings", "Table t {\n a int [primary key, null, unique, increment, default: -5]\n b text [not null, default: 'it\\'s', note: \"n\"]\n c bool [default: true]\n d text [default: null]\n}\n", func(t *testing.T, s *dbml.Schema) {
			a := s.Tables[0].Columns[0]
			require.True(t, a.PK && a.Unique && a.Increment && !a.NotNull)
			require.Equal(t, "-5", *a.Default)
			require.Equal(t, dbml.DefaultNumber, a.DefaultKind)
			b := s.Tables[0].Columns[1]
			require.Equal(t, "it's", *b.Default)
			require.Equal(t, dbml.DefaultString, b.DefaultKind)
			require.Equal(t, "n", b.Note)
			require.Equal(t, "true", *s.Tables[0].Columns[2].Default)
			require.Equal(t, "null", *s.Tables[0].Columns[3].Default)
		}},
		{"inline ref", "Table a {\n id int [pk]\n}\nTable b {\n a_id int [ref: > a.id]\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Len(t, s.Refs, 1)
			require.Equal(t, "b", s.Refs[0].From.Table)
			require.Equal(t, []string{"a_id"}, s.Refs[0].From.Columns)
			require.Equal(t, "a", s.Refs[0].To.Table)
		}},
		{"ref forms and every relation", "Table a {\n id int\n}\nTable b {\n id int\n}\nRef r1: a.id > b.id\nRef: a.id < b.id\nRef {\n a.id - b.id\n}\nRef named {\n a.id <> b.id\n}\nRef: a.id > b.id [delete: cascade]\n", func(t *testing.T, s *dbml.Schema) {
			var rels []string
			for _, r := range s.Refs {
				rels = append(rels, r.Relation)
			}
			require.Equal(t, []string{">", "<", "-", "<>", ">"}, rels)
			require.Equal(t, "r1", s.Refs[0].Name)
			require.Equal(t, "named", s.Refs[3].Name)
		}},
		{"composite ref and schema qualified endpoint", "Table x.a {\n p int\n q int\n}\nTable b {\n p int\n q int\n}\nRef: x.a.(p, q) > b.(p, q)\nRef: x.a.p > b.p\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, []string{"p", "q"}, s.Refs[0].From.Columns)
			require.Equal(t, "x.a", s.Refs[0].From.Table)
			require.Equal(t, "x.a", s.Refs[1].From.Table)
			require.Equal(t, "x.a.(p, q)", s.Refs[0].From.String())
		}},
		{"indexes", "Table t {\n a int\n b int\n indexes {\n  a\n  (a, b) [unique, name: 'ab_uq', type: hash]\n  `lower(a)` [note: 'x']\n  (a, b) [pk]\n }\n}\n", func(t *testing.T, s *dbml.Schema) {
			idx := s.Tables[0].Indexes
			require.Len(t, idx, 4)
			require.Equal(t, []dbml.IndexColumn{{Name: "a"}, {Name: "b"}}, idx[1].Columns)
			require.True(t, idx[1].Unique)
			require.Equal(t, "ab_uq", idx[1].Name)
			require.Equal(t, "hash", idx[1].Type)
			require.True(t, idx[2].Columns[0].Expr)
			require.True(t, idx[3].PK)
		}},
		{"enum values with notes and quoted names", "Enum \"my enum\" {\n a [note: 'first']\n \"b c\"\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "my enum", s.Enums[0].Name)
			require.Equal(t, "first", s.Enums[0].Values[0].Note)
			require.Equal(t, "b c", s.Enums[0].Values[1].Name)
		}},
		{"table groups", "Table a {\n id int\n}\nTable s.b {\n id int\n}\nTableGroup g [color: #fff] {\n a\n s.b\n note: 'n'\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "g", s.Groups[0].Name)
			require.Equal(t, []string{"a", "s.b"}, s.Groups[0].Tables)
		}},
		{"project and sticky notes are skipped", "Project p {\n database_type: 'PostgreSQL'\n Note: 'x'\n}\nNote hello {\n 'a'\n}\nTable t {\n id int\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Len(t, s.Tables, 1)
		}},
		{"all comment and string forms", "// line\n/* block\n over lines */\nTable t { // after\n a int [note: 'single']\n b int [note: \"double\"]\n c int [note: '''triple''']\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "triple", s.Tables[0].Columns[2].Note)
		}},
		{"crlf line endings and a BOM", "\xEF\xBB\xBFTable t {\r\n  id int [pk]\r\n  name text\r\n}\r\nRef: t.id > t.id\r\n", func(t *testing.T, s *dbml.Schema) {
			require.Len(t, s.Tables[0].Columns, 2)
			require.Len(t, s.Refs, 1)
		}},
		{"unicode and quoted identifiers with spaces", "Table \"my table\" {\n \"first name\" text\n имя varchar\n}\nRef: \"my table\".\"first name\" > \"my table\".имя\n", func(t *testing.T, s *dbml.Schema) {
			require.Equal(t, "my table", s.Tables[0].Name)
			require.Equal(t, "first name", s.Tables[0].Columns[0].Name)
			require.Equal(t, "имя", s.Tables[0].Columns[1].Name)
		}},
		{"one-line table", "Table t { id int [pk] }\n", func(t *testing.T, s *dbml.Schema) {
			require.Len(t, s.Tables[0].Columns, 1)
		}},
		{"keywords are case insensitive", "table t {\n id int [PK, Not Null]\n}\nENUM e {\n a\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.True(t, s.Tables[0].Columns[0].PK)
			require.True(t, s.Tables[0].Columns[0].NotNull)
		}},
		{"empty and comment-only input", "// nothing here\n", func(t *testing.T, s *dbml.Schema) {
			require.Empty(t, s.Tables)
		}},
		{"a ref may point at a table declared later", "Ref: a.id > b.id\nTable a {\n id int\n}\nTable b {\n id int\n}\n", func(t *testing.T, s *dbml.Schema) {
			require.Len(t, s.Refs, 1)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.check(t, mustParse(t, c.src))
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // "line:col message-substring"
	}{
		{"unterminated brace", "Table t {\n id int\n", []string{"1:7 Table 't' is not closed"}},
		{"unterminated string", "Table t {\n a text [note: 'oops]\n b int\n}\n", []string{"2:16 unterminated string", "3:2 expected ']'"}},
		{"unterminated comment", "Table t {\n id int\n}\n/* never closed", []string{"4:1 unterminated comment"}},
		{"missing bracket reports once and keeps going", "Table a {\n id int [pk\n}\nTable b {\n x int\n}\n", []string{"3:1 expected ']'"}},
		{"several independent mistakes are all reported", "Tabel a {\n id int\n}\nTable b {\n x [pk]\n y int [bogus]\n}\nRef: b.x > nope.id\n", []string{"1:1 unexpected", "5:2 column 'x' needs a type", "6:9 unknown column setting", "8:12 table 'nope' does not exist"}},
		{"column missing a type", "Table t {\n id\n}\n", []string{"2:2 column 'id' needs a type"}},
		{"duplicate table", "Table t {\n id int\n}\nTable t {\n id int\n}\n", []string{"4:7 table 't' is already defined"}},
		{"duplicate column", "Table t {\n id int\n id text\n}\n", []string{"3:2 column 'id' is already defined"}},
		{"ref to an unknown column", "Table a {\n id int\n}\nRef: a.nope > a.id\n", []string{"4:6 column 'nope' does not exist in table 'a'"}},
		{"ref to an unknown table", "Table a {\n id int\n}\nRef: a.id > ghost.id\n", []string{"4:13 table 'ghost' does not exist"}},
		{"bad relation symbol", "Table a {\n id int\n}\nRef: a.id = a.id\n", []string{"4:11 unexpected character", "4:13 expected a relation"}},
		{"composite refs need equal widths", "Table a {\n p int\n q int\n}\nRef: a.(p, q) > a.p\n", []string{"5:1 both sides of a relationship need the same number of columns"}},
		{"unexpected character", "Table t {\n id int @\n}\n", []string{"2:9 unexpected character"}},
		{"junk after a column", "Table t {\n id int [pk] extra\n}\n", []string{"2:14 unexpected 'extra'"}},
		{"unknown group member", "Table a {\n id int\n}\nTableGroup g {\n a\n ghost\n}\n", []string{"6:2 table 'ghost' does not exist"}},
		{"index on a missing column", "Table t {\n id int\n indexes {\n  nope\n }\n}\n", []string{"4:3 index column 'nope'"}},
		{"stray closing brace", "}\nTable t {\n id int\n}\n", []string{"1:1 unexpected '}'"}},
		{"bad default", "Table t {\n a int [default: foo]\n}\n", []string{"2:18 invalid default"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, errs := dbml.Parse(c.src)
			require.NotNil(t, s)

			got := make([]string, 0, len(errs))
			for _, e := range errs {
				got = append(got, fmt.Sprintf("%d:%d %s", e.Line, e.Column, e.Message))
			}
			require.Len(t, got, len(c.want), "errors: %v", got)
			for i, w := range c.want {
				require.Contains(t, got[i], w)
			}
		})
	}
}

func TestErrorsAreSortedAndPositionsAreCharacters(t *testing.T) {
	_, errs := dbml.Parse("Table t {\n имя int [bogus]\n}\nRef: a.b > c.d\n")
	require.Equal(t, "2:11", positions(errs)[0], "columns count characters, not bytes")
	for i := 1; i < len(errs); i++ {
		require.True(t, errs[i-1].Line < errs[i].Line || errs[i-1].Line == errs[i].Line && errs[i-1].Column <= errs[i].Column)
	}
	require.Contains(t, errs[0].Error(), "2:")
}

func TestErrorCountIsCapped(t *testing.T) {
	_, errs := dbml.Parse(strings.Repeat("@\n", 5000))
	require.LessOrEqual(t, len(errs), 100)
}

func TestParseNeverPanicsOrHangs(t *testing.T) {
	seeds := []string{sample, "Table a {\n id int [pk, ref: > b.id]\n indexes {\n (a, b) [unique]\n }\n}\nEnum e {\n a\n}\nRef { a.id > b.id }\nTableGroup g {\n a\n}\nProject p {\n x: 'y'\n}\n"}
	tokens := []string{"Table", "Enum", "Ref", "TableGroup", "Project", "Note", "indexes", "{", "}", "[", "]", "(", ")", ",", ":", ".", "<", ">", "<>", "-", "'", "\"", "`", "'''", "//", "/*", "*/", "\n", "\r\n", " ", "id", "int", "pk", "default", "ref", "as", "#fff", "12", "\\", "\x00", "é"}
	rng := rand.New(rand.NewSource(1))

	for i := range 3000 {
		var src string

		switch i % 3 {
		case 0: // truncated real input
			seed := seeds[rng.Intn(len(seeds))]
			src = seed[:rng.Intn(len(seed)+1)]
		case 1: // random token soup
			var b strings.Builder
			for range rng.Intn(60) {
				b.WriteString(tokens[rng.Intn(len(tokens))] + " ")
			}
			src = b.String()
		default: // random bytes
			raw := make([]byte, rng.Intn(80))
			rng.Read(raw)
			src = string(raw)
		}

		require.NotPanics(t, func() {
			s, _ := dbml.Parse(src)
			_ = s.Summary()
			_, _ = dbml.ExportSQL(s, dbml.Postgres)
			_, _ = dbml.ExportSQL(s, dbml.MySQL)
		}, "input: %q", src)
	}
}

func TestParseIsLinearOnLargeInput(t *testing.T) {
	var b strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&b, "Table t%d {\n id int [pk]\n name varchar(20) [not null]\n}\n", i)
	}
	for i := range 5000 {
		fmt.Fprintf(&b, "Ref: t%d.id > t%d.id\n", i, i+1)
	}

	s, errs := dbml.Parse(b.String())
	require.Empty(t, errs)
	require.Len(t, s.Tables, 20000)

	// Deeply nested and endless input must not recurse or loop.
	_, _ = dbml.Parse(strings.Repeat("{", 200000))
	_, _ = dbml.Parse("Table t {" + strings.Repeat("[", 100000))
	_, _ = dbml.Parse(strings.Repeat("Table ", 100000))
}

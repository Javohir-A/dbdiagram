package dbml

import (
	"strings"
)

type parser struct {
	toks []token
	pos  int
	errs *errList
	s    *Schema
}

func (p *parser) cur() token { return p.toks[p.pos] }

func (p *parser) peek(n int) token {
	i := p.pos + n
	if i >= len(p.toks) {
		i = len(p.toks) - 1
	}

	return p.toks[i]
}

// advance never moves past the EOF token, so cur is always valid.
func (p *parser) advance() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}

	return t
}

func (p *parser) prevLine() int {
	if p.pos == 0 {
		return 0
	}

	return p.toks[p.pos-1].endLine
}

func (p *parser) errorAt(t token, format string, args ...any) {
	p.errs.add(t.line, t.col, format, args...)
}

func (p *parser) expect(punct string) bool {
	if p.cur().isPunct(punct) {
		p.advance()
		return true
	}
	p.errorAt(p.cur(), "expected '%s' but found %s", punct, p.cur())

	return false
}

// skipLine drops the rest of a line, so one bad statement costs one error
// instead of derailing the block it is in.
func (p *parser) skipLine(line int) {
	for c := p.cur(); c.kind != tokEOF && c.line == line && !c.isPunct("}"); c = p.cur() {
		p.advance()
	}
}

// endStatement reports junk after a complete line statement.
func (p *parser) endStatement() {
	c := p.cur()
	if c.kind == tokEOF || c.isPunct("}") || c.line != p.prevLine() {
		return
	}
	p.errorAt(c, "unexpected %s; each item goes on its own line", c)
	p.skipLine(c.line)
}

func isTopKeyword(t token) bool {
	for _, k := range []string{"table", "enum", "ref", "tablegroup", "project", "note"} {
		if t.isWord(k) {
			return true
		}
	}

	return false
}

// recoverTop skips to the next top-level statement, keeping braces balanced so
// the body of a broken Table is not mistaken for new statements.
func (p *parser) recoverTop() {
	depth := 0

	for c := p.cur(); c.kind != tokEOF; c = p.cur() {
		switch {
		case c.isPunct("{"):
			depth++
		case c.isPunct("}"):
			depth = max(depth-1, 0)
		case depth == 0 && isTopKeyword(c):
			return
		}
		p.advance()
	}
}

func (p *parser) skipBlock() {
	open := p.advance()
	depth := 1

	for depth > 0 {
		c := p.cur()
		if c.kind == tokEOF {
			p.errorAt(open, "block is not closed: expected '}'")
			return
		}
		p.advance()

		switch {
		case c.isPunct("{"):
			depth++
		case c.isPunct("}"):
			depth--
		}
	}
}

func (p *parser) file() {
	for p.cur().kind != tokEOF {
		start := p.pos
		p.statement()
		if p.pos == start {
			p.advance()
		}
	}
}

func (p *parser) statement() {
	c := p.cur()

	switch {
	case c.isWord("table"):
		p.table()
	case c.isWord("enum"):
		p.enum()
	case c.isWord("ref"):
		p.ref()
	case c.isWord("tablegroup"):
		p.group()
	case c.isWord("project"):
		p.advance()
		if p.cur().isName() {
			p.advance()
		}
		if !p.cur().isPunct("{") {
			p.errorAt(p.cur(), "expected '{' after Project")
			p.recoverTop()
			return
		}
		p.skipBlock()
	case c.isWord("note"):
		p.advance()
		if p.cur().isName() && p.peek(1).isPunct("{") {
			p.advance()
		}
		switch {
		case p.cur().isPunct("{"):
			p.skipBlock()
		case p.cur().isPunct(":"):
			p.advance()
			if p.cur().isStr() {
				p.advance()
			}
		}
	case c.isPunct("}"):
		p.errorAt(c, "unexpected '}'")
		p.advance()
	default:
		p.errorAt(c, "unexpected %s; expected Table, Enum, Ref, TableGroup or Project", c)
		p.recoverTop()
	}
}

// qualifiedName reads name or schema.name.
func (p *parser) qualifiedName() ([]string, token, bool) {
	first := p.cur()
	if !first.isName() {
		p.errorAt(first, "expected a name but found %s", first)
		return nil, first, false
	}

	parts := []string{p.advance().text}
	for p.cur().isPunct(".") && p.peek(1).isName() {
		p.advance()
		parts = append(parts, p.advance().text)
	}

	return parts, first, true
}

func splitName(parts []string) (schema, name string) {
	name = parts[len(parts)-1]
	if len(parts) == 1 {
		return "public", name
	}

	return strings.Join(parts[:len(parts)-1], "."), name
}

type setting struct {
	key string
	val []token
	tok token
}

// settings reads `[a, b: value, c d]` after the opening bracket was seen.
func (p *parser) settings() []setting {
	var out []setting

	p.advance()

	for {
		c := p.cur()

		switch {
		case c.kind == tokEOF || c.isPunct("}"):
			p.errorAt(c, "expected ']' but found %s", c)
			return out
		case c.isPunct("]"):
			p.advance()
			return out
		case c.isPunct(","):
			p.advance()
			continue
		}

		s, ok := p.setting()
		if ok {
			out = append(out, s)

			// A token on a later line means the bracket was forgotten: report it
			// once and leave that line to the caller instead of eating it.
			if n := p.cur(); !n.isPunct(",") && !n.isPunct("]") && n.kind != tokEOF && !n.isPunct("}") {
				if n.line != p.prevLine() {
					p.errorAt(n, "expected ']' but found %s", n)
					return out
				}
				p.errorAt(n, "expected ',' or ']' but found %s", n)
			}
		}
		for n := p.cur(); n.kind != tokEOF && !n.isPunct(",") && !n.isPunct("]") && !n.isPunct("}"); n = p.cur() {
			p.advance()
		}
	}
}

func (p *parser) setting() (setting, bool) {
	c := p.cur()
	if c.kind != tokIdent {
		p.errorAt(c, "unexpected %s in settings", c)
		return setting{}, false
	}

	var words []string
	for p.cur().kind == tokIdent {
		words = append(words, strings.ToLower(p.advance().text))
	}

	s := setting{key: strings.Join(words, " "), tok: c}
	if p.cur().isPunct(":") {
		p.advance()
		s.val = p.settingValue()
	}

	return s, true
}

// settingValue collects the tokens up to the next comma or closing bracket;
// parentheses nest, so composite endpoints keep their commas. A value never
// continues on the next line.
func (p *parser) settingValue() []token {
	var (
		out   []token
		depth int
	)

	for c := p.cur(); c.kind != tokEOF && !c.isPunct("}"); c = p.cur() {
		if depth == 0 && (c.isPunct(",") || c.isPunct("]")) || c.line != p.prevLine() {
			break
		}

		switch {
		case c.isPunct("(") || c.isPunct("["):
			depth++
		case (c.isPunct(")") || c.isPunct("]")) && depth > 0:
			depth--
		}
		out = append(out, p.advance())
	}

	return out
}

func (p *parser) stringValue(s setting) string {
	if len(s.val) == 1 && s.val[0].isStr() {
		return s.val[0].text
	}
	p.errorAt(s.tok, "'%s' needs a quoted string", s.key)

	return ""
}

func (p *parser) table() {
	p.advance()

	parts, nameTok, ok := p.qualifiedName()
	if !ok {
		p.recoverTop()
		return
	}

	schema, name := splitName(parts)
	t := Table{Schema: schema, Name: name, at: nameTok.at()}

	if p.cur().isWord("as") && p.peek(1).isName() {
		p.advance()
		t.Alias = p.advance().text
	}
	if p.cur().isPunct("[") {
		for _, s := range p.settings() {
			switch s.key {
			case "headercolor":
				if len(s.val) == 1 {
					t.Color = s.val[0].text
				}
			case "note":
				t.Note = p.stringValue(s)
			default:
				p.errorAt(s.tok, "unknown table setting '%s'", s.key)
			}
		}
	}
	if !p.expect("{") {
		p.recoverTop()
		return
	}

	closed := false
	for p.cur().kind != tokEOF && !closed {
		start := p.pos
		if p.cur().isPunct("}") {
			p.advance()
			closed = true
			continue
		}
		p.tableItem(&t)
		if p.pos == start {
			p.advance()
		}
	}
	if !closed {
		p.errorAt(nameTok, "Table '%s' is not closed: expected '}'", name)
	}

	p.s.Tables = append(p.s.Tables, t)
}

func (p *parser) tableItem(t *Table) {
	c := p.cur()

	switch {
	case c.isWord("indexes") && p.peek(1).isPunct("{"):
		p.indexes(t)
	case c.isWord("note") && (p.peek(1).isPunct(":") || p.peek(1).isPunct("{")):
		t.Note = p.note()
	case c.isName():
		p.column(t)
	default:
		p.errorAt(c, "unexpected %s in Table '%s'", c, t.Name)
		p.skipLine(c.line)
	}
}

func (p *parser) note() string {
	p.advance()

	if p.cur().isPunct(":") {
		p.advance()
		if p.cur().isStr() {
			return p.advance().text
		}
		p.errorAt(p.cur(), "expected a quoted note but found %s", p.cur())

		return ""
	}

	p.advance()

	var text string
	if p.cur().isStr() {
		text = p.advance().text
	}
	p.expect("}")

	return text
}

func (p *parser) column(t *Table) {
	nameTok := p.advance()
	col := Column{Name: nameTok.text, at: nameTok.at()}

	typ, ok := p.colType(nameTok)
	col.Type = typ
	if !ok {
		t.Columns = append(t.Columns, col)
		p.skipLine(nameTok.endLine)
		return
	}

	if p.cur().isPunct("[") {
		for _, s := range p.settings() {
			p.columnSetting(t, &col, s)
		}
	}
	t.Columns = append(t.Columns, col)
	p.endStatement()
}

func (p *parser) columnSetting(t *Table, col *Column, s setting) {
	switch s.key {
	case "pk", "primary key":
		col.PK = true
	case "not null":
		col.NotNull = true
	case "null":
		col.NotNull = false
	case "unique":
		col.Unique = true
	case "increment":
		col.Increment = true
	case "note":
		col.Note = p.stringValue(s)
	case "default":
		p.setDefault(col, s)
	case "ref":
		p.inlineRef(t, col, s)
	default:
		p.errorAt(s.tok, "unknown column setting '%s'", s.key)
	}
}

func (p *parser) setDefault(col *Column, s setting) {
	var (
		v      = s.val
		prefix string
	)

	if len(v) == 2 && v[0].isPunct("-") && v[1].kind == tokNumber {
		prefix, v = "-", v[1:]
	}
	if len(v) != 1 {
		p.errorAt(s.tok, "'default' needs one value")
		return
	}

	var (
		t    = v[0]
		text = prefix + t.text
		kind DefaultKind
	)

	switch {
	case t.isStr():
		kind = DefaultString
	case t.kind == tokExpr:
		kind = DefaultExpr
	case t.kind == tokNumber:
		kind = DefaultNumber
	case t.isWord("true") || t.isWord("false") || t.isWord("null"):
		kind, text = DefaultNumber, strings.ToLower(t.text)
	default:
		p.errorAt(t, "invalid default value %s", t)
		return
	}

	col.Default, col.DefaultKind = &text, kind
}

// colType reads the type on the same line as the column name.
func (p *parser) colType(nameTok token) (string, bool) {
	c := p.cur()
	if !c.isName() || c.line != nameTok.endLine {
		p.errorAt(nameTok, "column '%s' needs a type", nameTok.text)
		return "", false
	}

	var b strings.Builder

	b.WriteString(p.advance().text)
	for p.cur().isPunct(".") && p.peek(1).isName() {
		p.advance()
		b.WriteString("." + p.advance().text)
	}

	if p.cur().isPunct("(") {
		p.advance()
		b.WriteString("(")

		for first := true; ; first = false {
			c := p.cur()
			if c.isPunct(")") {
				p.advance()
				break
			}
			if !first {
				if !c.isPunct(",") {
					p.errorAt(c, "expected ',' or ')' in the type arguments but found %s", c)
					return b.String(), false
				}
				p.advance()
				b.WriteString(",")
				c = p.cur()
			}

			switch {
			case c.kind == tokNumber || c.kind == tokIdent:
				b.WriteString(p.advance().text)
			case c.isStr():
				b.WriteString("'" + strings.ReplaceAll(p.advance().text, "'", "''") + "'")
			default:
				p.errorAt(c, "unexpected %s in the type arguments", c)
				return b.String(), false
			}
		}
		b.WriteString(")")
	}

	// `int[]` and `int[3]` are arrays; `int [pk]` starts the settings.
	for p.cur().isPunct("[") && (p.peek(1).isPunct("]") || p.peek(1).kind == tokNumber && p.peek(2).isPunct("]")) {
		p.advance()
		b.WriteString("[")
		if p.cur().kind == tokNumber {
			b.WriteString(p.advance().text)
		}
		p.advance()
		b.WriteString("]")
	}

	return b.String(), true
}

func (p *parser) indexes(t *Table) {
	p.advance()
	open := p.advance()

	for {
		c := p.cur()
		if c.kind == tokEOF {
			p.errorAt(open, "indexes block is not closed: expected '}'")
			return
		}
		if c.isPunct("}") {
			p.advance()
			return
		}

		start := p.pos
		p.index(t)
		if p.pos == start {
			p.advance()
		}
	}
}

func (p *parser) index(t *Table) {
	first := p.cur()
	idx := Index{at: first.at()}

	readCol := func() bool {
		c := p.cur()

		switch {
		case c.kind == tokExpr:
			idx.Columns = append(idx.Columns, IndexColumn{Name: p.advance().text, Expr: true})
		case c.isName():
			idx.Columns = append(idx.Columns, IndexColumn{Name: p.advance().text})
		default:
			p.errorAt(c, "expected a column or `expression` but found %s", c)
			return false
		}

		return true
	}

	if first.isPunct("(") {
		p.advance()

		for {
			if !readCol() {
				p.skipLine(first.line)
				return
			}
			if p.cur().isPunct(",") {
				p.advance()
				continue
			}
			if !p.expect(")") {
				p.skipLine(first.line)
				return
			}
			break
		}
	} else if !readCol() {
		p.skipLine(first.line)
		return
	}

	if p.cur().isPunct("[") {
		for _, s := range p.settings() {
			switch s.key {
			case "unique":
				idx.Unique = true
			case "pk", "primary key":
				idx.PK = true
			case "name":
				idx.Name = p.stringValue(s)
			case "note":
				idx.Note = p.stringValue(s)
			case "type":
				if len(s.val) == 1 {
					idx.Type = strings.ToLower(s.val[0].text)
				}
			default:
				p.errorAt(s.tok, "unknown index setting '%s'", s.key)
			}
		}
	}

	t.Indexes = append(t.Indexes, idx)
	p.endStatement()
}

func (p *parser) enum() {
	p.advance()

	parts, nameTok, ok := p.qualifiedName()
	if !ok {
		p.recoverTop()
		return
	}
	if !p.expect("{") {
		p.recoverTop()
		return
	}

	schema, name := splitName(parts)
	e := Enum{Schema: schema, Name: name, at: nameTok.at()}
	closed := false

	for p.cur().kind != tokEOF && !closed {
		c := p.cur()

		switch {
		case c.isPunct("}"):
			p.advance()
			closed = true
		case c.isName():
			p.advance()

			v := EnumValue{Name: c.text}
			if p.cur().isPunct("[") {
				for _, s := range p.settings() {
					if s.key != "note" {
						p.errorAt(s.tok, "unknown enum value setting '%s'", s.key)
						continue
					}
					v.Note = p.stringValue(s)
				}
			}
			e.Values = append(e.Values, v)
			p.endStatement()
		default:
			p.errorAt(c, "unexpected %s in Enum '%s'", c, name)
			p.advance()
			p.skipLine(c.line)
		}
	}
	if !closed {
		p.errorAt(nameTok, "Enum '%s' is not closed: expected '}'", name)
	}

	p.s.Enums = append(p.s.Enums, e)
}

func (p *parser) group() {
	p.advance()

	parts, nameTok, ok := p.qualifiedName()
	if !ok {
		p.recoverTop()
		return
	}

	g := Group{Name: strings.Join(parts, "."), pos: nameTok.at()}
	if p.cur().isPunct("[") {
		p.settings()
	}
	if !p.expect("{") {
		p.recoverTop()
		return
	}

	closed := false
	for p.cur().kind != tokEOF && !closed {
		c := p.cur()

		switch {
		case c.isPunct("}"):
			p.advance()
			closed = true
		case c.isWord("note") && (p.peek(1).isPunct(":") || p.peek(1).isPunct("{")):
			p.note()
		case c.isName():
			member, tok, _ := p.qualifiedName()
			g.Tables = append(g.Tables, strings.Join(member, "."))
			g.at = append(g.at, tok.at())
		default:
			p.errorAt(c, "unexpected %s in TableGroup '%s'", c, g.Name)
			p.advance()
			p.skipLine(c.line)
		}
	}
	if !closed {
		p.errorAt(nameTok, "TableGroup '%s' is not closed: expected '}'", g.Name)
	}

	p.s.Groups = append(p.s.Groups, g)
}

func (p *parser) ref() {
	kw := p.advance()
	name := ""

	if p.cur().isName() && (p.peek(1).isPunct(":") || p.peek(1).isPunct("{")) {
		name = p.advance().text
	}

	switch {
	case p.cur().isPunct(":"):
		p.advance()
		p.refBody(name, kw)
	case p.cur().isPunct("{"):
		open := p.advance()

		for {
			c := p.cur()
			if c.kind == tokEOF {
				p.errorAt(open, "Ref is not closed: expected '}'")
				return
			}
			if c.isPunct("}") {
				p.advance()
				return
			}

			start := p.pos
			p.refBody(name, kw)
			if p.pos == start {
				p.advance()
			}
		}
	default:
		p.errorAt(p.cur(), "expected ':' or '{' after Ref but found %s", p.cur())
		p.recoverTop()
	}
}

func (p *parser) refBody(name string, kw token) {
	from, ok := p.endpoint(false)
	if !ok {
		p.skipLine(p.prevLine())
		return
	}
	rel, ok := p.relation()
	if !ok {
		p.skipLine(p.prevLine())
		return
	}
	to, ok := p.endpoint(true)
	if !ok {
		p.skipLine(p.prevLine())
		return
	}

	if p.cur().isPunct("[") && p.cur().line == p.prevLine() {
		p.settings() // on delete / on update / color: not modelled
	}

	p.s.Refs = append(p.s.Refs, Ref{Name: name, From: from, To: to, Relation: rel, at: kw.at()})
	p.endStatement()
}

func (p *parser) inlineRef(t *Table, col *Column, s setting) {
	if len(s.val) == 0 {
		p.errorAt(s.tok, "'ref' needs a relation and a column, like ref: > users.id")
		return
	}

	sub := &parser{toks: append(append([]token{}, s.val...), token{kind: tokEOF, line: s.val[0].line, col: s.val[0].col, endLine: s.val[0].line}), errs: p.errs, s: p.s}

	rel, ok := sub.relation()
	if !ok {
		return
	}
	to, ok := sub.endpoint(true)
	if !ok {
		return
	}
	if c := sub.cur(); c.kind != tokEOF {
		p.errorAt(c, "unexpected %s after the ref", c)
		return
	}

	from := Endpoint{Table: t.ID(), Columns: []string{col.Name}, at: s.tok.at()}
	p.s.Refs = append(p.s.Refs, Ref{From: from, To: to, Relation: rel, at: s.tok.at()})
}

func (p *parser) relation() (string, bool) {
	c := p.cur()
	if c.isPunct("<") || c.isPunct(">") || c.isPunct("-") || c.isPunct("<>") {
		p.advance()
		return c.text, true
	}
	p.errorAt(c, "expected a relation (<, >, - or <>) but found %s", c)

	return "", false
}

// endpoint reads table.column, schema.table.column or table.(a, b).
func (p *parser) endpoint(sameLine bool) (Endpoint, bool) {
	first := p.cur()
	if sameLine && p.pos > 0 && first.line != p.prevLine() {
		p.errorAt(p.toks[p.pos-1], "expected table.column after the relation")
		return Endpoint{}, false
	}

	var (
		parts []string
		cols  []string
	)

	for {
		c := p.cur()
		if !c.isName() {
			p.errorAt(c, "expected table.column but found %s", c)
			return Endpoint{}, false
		}
		parts = append(parts, p.advance().text)

		if !p.cur().isPunct(".") {
			break
		}
		p.advance()

		if p.cur().isPunct("(") {
			p.advance()

			for {
				n := p.cur()
				if !n.isName() {
					p.errorAt(n, "expected a column name but found %s", n)
					return Endpoint{}, false
				}
				cols = append(cols, p.advance().text)
				if p.cur().isPunct(",") {
					p.advance()
					continue
				}
				if !p.expect(")") {
					return Endpoint{}, false
				}
				break
			}
			break
		}
	}

	if cols == nil {
		if len(parts) < 2 {
			p.errorAt(first, "expected table.column but found only %s", first)
			return Endpoint{}, false
		}
		cols, parts = []string{parts[len(parts)-1]}, parts[:len(parts)-1]
	}

	return Endpoint{Table: strings.Join(parts, "."), Columns: cols, at: first.at()}, true
}

// check runs the semantic rules once the whole document is read, so a Ref may
// point at a table declared later.
func (p *parser) check() {
	var (
		s       = p.s
		ids     = map[string]*Table{}
		aliases = map[string]*Table{}
	)

	for i := range s.Tables {
		t := &s.Tables[i]
		if _, dup := ids[t.ID()]; dup {
			p.errs.add(t.at.line, t.at.col, "table '%s' is already defined", t.ID())
			continue
		}
		ids[t.ID()] = t

		cols := map[string]bool{}
		for _, c := range t.Columns {
			if cols[c.Name] {
				p.errs.add(c.at.line, c.at.col, "column '%s' is already defined in table '%s'", c.Name, t.ID())
			}
			cols[c.Name] = true
		}
	}
	for i := range s.Tables {
		if a := s.Tables[i].Alias; a != "" && aliases[a] == nil {
			aliases[a] = &s.Tables[i]
		}
	}

	resolve := func(name string) *Table {
		name = strings.TrimPrefix(name, "public.")
		if t := ids[name]; t != nil {
			return t
		}

		return aliases[name]
	}

	for i := range s.Refs {
		r := &s.Refs[i]
		p.checkEndpoint(&r.From, resolve)
		p.checkEndpoint(&r.To, resolve)
		if len(r.From.Columns) != len(r.To.Columns) {
			p.errs.add(r.at.line, r.at.col, "both sides of a relationship need the same number of columns")
		}
	}

	for i := range s.Tables {
		t := &s.Tables[i]
		for _, idx := range t.Indexes {
			for _, ic := range idx.Columns {
				if !ic.Expr && !hasColumn(t, ic.Name) {
					p.errs.add(idx.at.line, idx.at.col, "index column '%s' does not exist in table '%s'", ic.Name, t.ID())
				}
			}
		}
	}

	for i := range s.Groups {
		g := &s.Groups[i]
		for j, member := range g.Tables {
			t := resolve(member)
			if t == nil {
				p.errs.add(g.at[j].line, g.at[j].col, "table '%s' does not exist", member)
				continue
			}
			g.Tables[j] = t.ID()
		}
	}
}

func (p *parser) checkEndpoint(ep *Endpoint, resolve func(string) *Table) {
	t := resolve(ep.Table)
	if t == nil {
		p.errs.add(ep.at.line, ep.at.col, "table '%s' does not exist", ep.Table)
		return
	}

	ep.Table = t.ID()
	for _, c := range ep.Columns {
		if !hasColumn(t, c) {
			p.errs.add(ep.at.line, ep.at.col, "column '%s' does not exist in table '%s'", c, t.ID())
		}
	}
}

func hasColumn(t *Table, name string) bool {
	for _, c := range t.Columns {
		if c.Name == name {
			return true
		}
	}

	return false
}

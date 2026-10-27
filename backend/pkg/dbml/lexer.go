package dbml

import (
	"strings"
	"unicode"
)

type tokKind int

const (
	tokEOF    tokKind = iota
	tokIdent          // bare word
	tokQuoted         // "double quoted": a name, or a string where a value is expected
	tokString         // 'single' or '''triple'''
	tokExpr           // `backtick`
	tokNumber
	tokColor // #3498db
	tokPunct // text holds the symbol
)

type token struct {
	kind    tokKind
	text    string
	line    int
	col     int
	endLine int
}

func (t token) isPunct(s string) bool { return t.kind == tokPunct && t.text == s }
func (t token) isWord(s string) bool  { return t.kind == tokIdent && strings.EqualFold(t.text, s) }
func (t token) isName() bool          { return t.kind == tokIdent || t.kind == tokQuoted }
func (t token) isStr() bool           { return t.kind == tokString || t.kind == tokQuoted }
func (t token) at() pos               { return pos{t.line, t.col} }

func (t token) String() string {
	switch t.kind {
	case tokEOF:
		return "end of file"
	case tokString, tokQuoted:
		return "\"" + oneLine(t.text) + "\""
	case tokExpr:
		return "`" + oneLine(t.text) + "`"
	default:
		return "'" + oneLine(t.text) + "'"
	}
}

type lexer struct {
	rs   []rune
	i    int
	line int
	col  int
	toks []token
	errs *errList
}

// lex never fails: a bad character or an unterminated string is reported and
// skipped, so the parser still sees the rest of the document.
func lex(src string, errs *errList) []token {
	l := &lexer{rs: []rune(src), line: 1, col: 1, errs: errs}

	for l.skipSpace(); l.i < len(l.rs); l.skipSpace() {
		l.next()
	}
	l.toks = append(l.toks, token{kind: tokEOF, line: l.line, col: l.col, endLine: l.line})

	return l.toks
}

func (l *lexer) at(n int) rune {
	if l.i+n >= len(l.rs) {
		return 0
	}

	return l.rs[l.i+n]
}

func (l *lexer) advance() rune {
	r := l.rs[l.i]
	l.i++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}

	return r
}

func (l *lexer) skipSpace() {
	for l.i < len(l.rs) {
		r := l.rs[l.i]

		switch {
		case unicode.IsSpace(r) || r == 0xFEFF:
			l.advance()
		case r == '/' && l.at(1) == '/':
			for l.i < len(l.rs) && l.rs[l.i] != '\n' {
				l.advance()
			}
		case r == '/' && l.at(1) == '*':
			line, col := l.line, l.col
			l.advance()
			l.advance()

			closed := false
			for l.i < len(l.rs) && !closed {
				closed = l.rs[l.i] == '*' && l.at(1) == '/'
				if closed {
					l.advance()
				}
				l.advance()
			}
			if !closed {
				l.errs.add(line, col, "unterminated comment")
			}
		default:
			return
		}
	}
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_'
}

func (l *lexer) next() {
	var (
		line, col = l.line, l.col
		r         = l.rs[l.i]
	)

	emit := func(kind tokKind, text string) {
		l.toks = append(l.toks, token{kind: kind, text: text, line: line, col: col, endLine: l.line})
	}

	switch {
	case r == '\'' && l.at(1) == '\'' && l.at(2) == '\'':
		emit(tokString, l.triple(line, col))
	case r == '\'':
		emit(tokString, l.quoted('\'', line, col))
	case r == '"':
		emit(tokQuoted, l.quoted('"', line, col))
	case r == '`':
		emit(tokExpr, l.quoted('`', line, col))
	case r == '#':
		start := l.i
		l.advance()
		for l.i < len(l.rs) && isIdentPart(l.rs[l.i]) {
			l.advance()
		}
		emit(tokColor, string(l.rs[start:l.i]))
	case isIdentStart(r) || unicode.IsDigit(r):
		emit(l.word())
	case r == '<' && l.at(1) == '>':
		l.advance()
		l.advance()
		emit(tokPunct, "<>")
	case strings.ContainsRune("{}[](),:.<>-", r):
		l.advance()
		emit(tokPunct, string(r))
	default:
		l.advance()
		l.errs.add(line, col, "unexpected character %q", r)
	}
}

// word reads an identifier or a number. A run that starts with a digit and
// contains a letter (2fa_codes) is an identifier; only all digits, optionally
// with a fraction, is a number.
func (l *lexer) word() (tokKind, string) {
	start := l.i
	for l.i < len(l.rs) && isIdentPart(l.rs[l.i]) {
		l.advance()
	}

	text := string(l.rs[start:l.i])
	if !unicode.IsDigit(l.rs[start]) {
		return tokIdent, text
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return tokIdent, text
		}
	}

	if l.i < len(l.rs) && l.rs[l.i] == '.' && unicode.IsDigit(l.at(1)) {
		l.advance()
		for l.i < len(l.rs) && unicode.IsDigit(l.rs[l.i]) {
			l.advance()
		}
		text = string(l.rs[start:l.i])
	}

	return tokNumber, text
}

// quoted reads a one-line string. Strings and names never span lines, so an
// unterminated one stops at the line end and the next statement still parses.
func (l *lexer) quoted(delim rune, line, col int) string {
	var b strings.Builder

	l.advance()

	for l.i < len(l.rs) {
		r := l.rs[l.i]

		switch {
		case r == delim:
			l.advance()
			return b.String()
		case r == '\n':
			l.errs.add(line, col, "unterminated string")
			return b.String()
		case r == '\\' && delim != '`':
			l.advance()
			if l.i >= len(l.rs) || l.rs[l.i] == '\n' {
				continue
			}

			switch e := l.advance(); e {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			case 'r':
				b.WriteRune('\r')
			default:
				b.WriteRune(e)
			}
		default:
			b.WriteRune(l.advance())
		}
	}

	l.errs.add(line, col, "unterminated string")

	return b.String()
}

func (l *lexer) triple(line, col int) string {
	var b strings.Builder

	for range 3 {
		l.advance()
	}

	for l.i < len(l.rs) {
		if l.rs[l.i] == '\'' && l.at(1) == '\'' && l.at(2) == '\'' {
			for range 3 {
				l.advance()
			}

			return strings.TrimSpace(b.String())
		}
		b.WriteRune(l.advance())
	}

	l.errs.add(line, col, "unterminated string")

	return strings.TrimSpace(b.String())
}

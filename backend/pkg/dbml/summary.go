package dbml

import "strings"

// Summary renders the schema as compact text for people and language models:
// one line per table, then relationships, enums and groups, all in source
// order so equal schemas give equal text.
func (s *Schema) Summary() string {
	var blocks []string

	if len(s.Tables) > 0 {
		lines := make([]string, 0, len(s.Tables))
		for i := range s.Tables {
			lines = append(lines, tableLine(&s.Tables[i]))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	if len(s.Refs) > 0 {
		lines := []string{"relationships:"}
		for _, r := range s.Refs {
			lines = append(lines, r.From.String()+" "+r.Relation+" "+r.To.String())
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	if len(s.Enums) > 0 {
		lines := []string{"enums:"}
		for i := range s.Enums {
			e := &s.Enums[i]
			values := make([]string, 0, len(e.Values))
			for _, v := range e.Values {
				values = append(values, v.Name)
			}
			lines = append(lines, e.ID()+": "+strings.Join(values, ", "))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	if len(s.Groups) > 0 {
		lines := []string{"table groups:"}
		for _, g := range s.Groups {
			lines = append(lines, g.Name+": "+strings.Join(g.Tables, ", "))
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	return strings.Join(blocks, "\n\n")
}

func (e Endpoint) String() string {
	if len(e.Columns) == 1 {
		return e.Table + "." + e.Columns[0]
	}

	return e.Table + ".(" + strings.Join(e.Columns, ", ") + ")"
}

func tableLine(t *Table) string {
	cols := make([]string, 0, len(t.Columns))

	for _, c := range t.Columns {
		parts := []string{c.Name, c.Type}
		if c.PK {
			parts = append(parts, "pk")
		}
		if c.Increment {
			parts = append(parts, "increment")
		}
		if c.NotNull {
			parts = append(parts, "not null")
		}
		if c.Unique {
			parts = append(parts, "unique")
		}
		if c.Default != nil {
			parts = append(parts, "default "+defaultText(&c))
		}
		cols = append(cols, strings.TrimSpace(strings.Join(parts, " ")))
	}

	line := t.ID() + "(" + strings.Join(cols, ", ") + ")"
	if t.Note != "" {
		line += " -- " + oneLine(t.Note)
	}

	return line
}

func defaultText(c *Column) string {
	switch c.DefaultKind {
	case DefaultString:
		return "'" + oneLine(*c.Default) + "'"
	case DefaultExpr:
		return "`" + oneLine(*c.Default) + "`"
	default:
		return *c.Default
	}
}

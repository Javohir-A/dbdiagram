package mcp

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/pkg/dbml"
)

// maxErrorsShown keeps a badly broken document from flooding the model.
const maxErrorsShown = 20

type dbmlSource struct {
	DiagramID string `json:"diagram_id"`
	DBML      string `json:"dbml"`
}

// source resolves the DBML a tool works on: the text given, or a diagram the
// caller can read. Exactly one of the two.
func (s *Server) source(c *gin.Context, who caller, a dbmlSource) (string, error) {
	switch {
	case a.DiagramID != "" && a.DBML != "":
		return "", badInput("send either diagram_id or dbml, not both")
	case a.DiagramID != "":
		d, err := s.app.Diagrams.Diagram.Get(c.Request.Context(), who.UserID, a.DiagramID)
		if err != nil {
			return "", err
		}

		return d.DBML, nil
	case a.DBML == "":
		return "", badInput("send diagram_id or dbml")
	case len(a.DBML) > config.MaxDBMLSize:
		return "", badInput("dbml must be at most %d bytes", config.MaxDBMLSize)
	}

	return a.DBML, nil
}

type errorView struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

func errorsView(errs []dbml.Error) []errorView {
	out := make([]errorView, 0, min(len(errs), maxErrorsShown))
	for _, e := range errs[:min(len(errs), maxErrorsShown)] {
		out = append(out, errorView{Line: e.Line, Column: e.Column, Message: e.Message})
	}

	return out
}

func (s *Server) dbmlTools() []*tool {
	either := props{
		"diagram_id": str("Use the DBML of this diagram (you need read access)."),
		"dbml":       str("Or DBML text to work on directly."),
	}

	return []*tool{
		{
			Name:        "validate_dbml",
			Description: "Check DBML for syntax and reference errors without saving anything. Returns every error with its line and column, so fix them all in one pass.",
			Schema:      object(props{"dbml": str("The DBML text to check (up to 1 MiB).")}, "dbml"),
			Run: func(_ *gin.Context, _ caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DBML string `json:"dbml"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if len(a.DBML) > config.MaxDBMLSize {
					return nil, badInput("dbml must be at most %d bytes", config.MaxDBMLSize)
				}

				schema, errs := dbml.Parse(a.DBML)

				return map[string]any{
					"valid": len(errs) == 0, "error_count": len(errs), "errors": errorsView(errs),
					"tables": len(schema.Tables), "refs": len(schema.Refs),
				}, nil
			},
		},
		{
			Name:        "describe_schema",
			Description: "Summarise the tables, columns, relationships and enums of a diagram (or of DBML text) in a compact form. Cheaper than reading the whole DBML when you only need the structure.",
			Schema:      object(either),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[dbmlSource](raw)
				if err != nil {
					return nil, err
				}
				src, err := s.source(c, who, a)
				if err != nil {
					return nil, err
				}

				schema, errs := dbml.Parse(src)

				out := map[string]any{"tables": len(schema.Tables), "refs": len(schema.Refs), "summary": schema.Summary()}
				if len(errs) > 0 {
					out["errors"] = errorsView(errs)
				}

				return out, nil
			},
		},
		{
			Name:        "export_sql",
			Description: "Generate CREATE TABLE / index / foreign key SQL for PostgreSQL or MySQL from a diagram (or from DBML text). Comment lines in the output mark anything that could not be translated. Nothing is executed.",
			Schema:      object(with2(either, props{"dialect": oneOf("Target database.", "postgres", "mysql")}), "dialect"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					dbmlSource
					Dialect string `json:"dialect"`
				}](raw)
				if err != nil {
					return nil, err
				}

				d := dbml.Dialect(strings.ToLower(a.Dialect))
				if d != dbml.Postgres && d != dbml.MySQL {
					return nil, badInput("dialect must be postgres or mysql")
				}

				src, err := s.source(c, who, a.dbmlSource)
				if err != nil {
					return nil, err
				}

				schema, errs := dbml.Parse(src)
				if len(errs) > 0 {
					return nil, badInput("the DBML has %d error(s), first: line %d: %s. Fix it (see validate_dbml) before exporting.", len(errs), errs[0].Line, errs[0].Message)
				}

				sql, err := dbml.ExportSQL(schema, d)
				if err != nil {
					return nil, badInput("%v", err)
				}

				return map[string]any{"dialect": a.Dialect, "sql": sql}, nil
			},
		},
	}
}

func with2(a, b props) props {
	out := props{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}

	return out
}

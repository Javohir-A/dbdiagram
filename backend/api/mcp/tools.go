package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// tool is one capability. Run gets the arguments already decoded into its own
// type by `bind`, and returns a value that is shown to the model as JSON text.
type tool struct {
	Name        string
	Description string
	Schema      map[string]any
	// Write marks a tool that changes something: it is refused for read-only
	// tokens. Destructive marks the subset that deletes or overwrites.
	Write       bool
	Destructive bool
	Run         func(c *gin.Context, who caller, args json.RawMessage) (any, error)
}

func (s *Server) listTools() any {
	out := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.Schema,
			"annotations": map[string]any{
				"readOnlyHint":    !t.Write,
				"destructiveHint": t.Destructive,
				"openWorldHint":   false,
			},
		})
	}

	return map[string]any{"tools": out}
}

func (s *Server) callTool(c *gin.Context, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params"}
	}

	t, ok := s.index[p.Name]
	if !ok {
		return nil, &rpcError{Code: codeInvalidParams, Message: errUnknownTool.Error() + ": " + p.Name}
	}

	who := callerOf(c)
	if t.Write && who.ReadOnly {
		return toolError("this token is read-only; create a token with write access to use " + t.Name), nil
	}
	if len(bytes.TrimSpace(p.Arguments)) == 0 {
		p.Arguments = json.RawMessage("{}")
	}

	res, err := t.Run(c, who, p.Arguments)
	if err != nil {
		return s.failed(c, t, err), nil
	}

	text, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return s.failed(c, t, err), nil
	}

	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(text)}}}, nil
}

// failed turns an error into a tool result the model can act on. Anything that
// is not a deliberate apperr message is hidden, like a 500 over REST.
func (s *Server) failed(c *gin.Context, t *tool, err error) any {
	var inputErr *inputError
	if errors.As(err, &inputErr) {
		return toolError(inputErr.Error())
	}

	_, e := apperr.HTTPStatus(err)
	if e.Kind == apperr.KindInternal || e.Kind == apperr.KindUnavailable {
		s.log.Error("mcp tool failed", logger.String("tool", t.Name), logger.String("request_id", c.GetString("request_id")), logger.Error(e.Err))
	}

	msg := e.Message
	if e.Kind == apperr.KindRevisionConflict {
		msg += ". Call get_diagram again, merge your change into the latest DBML, and retry with the new revision."
	}

	return toolError(msg)
}

func toolError(message string) map[string]any {
	return map[string]any{"isError": true, "content": []map[string]any{{"type": "text", "text": message}}}
}

// inputError is a problem with the arguments the model sent.
type inputError struct{ msg string }

func (e *inputError) Error() string { return e.msg }

func badInput(format string, a ...any) error { return &inputError{msg: fmt.Sprintf(format, a...)} }

// bind decodes tool arguments strictly: an unknown field is reported, so a
// model that misspells `base_revision` is told instead of silently losing the
// conflict protection.
func bind[T any](raw json.RawMessage) (T, error) {
	var v T

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, badInput("invalid arguments: %v", err)
	}

	return v, nil
}

// ---- schema helpers: just enough JSON Schema, written once --------------------

type props map[string]any

func object(p props, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": p, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}

	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func oneOf(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

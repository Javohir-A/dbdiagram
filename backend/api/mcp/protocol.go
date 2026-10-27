// Package mcp serves the Model Context Protocol (Streamable HTTP, stateless
// JSON-RPC 2.0) so AI agents can use every feature of the app. It is an HTTP
// adapter like the REST handlers: it parses a request, calls ONE service method
// as the caller, and shapes the answer. Authorization lives in the services,
// so a tool can never do more than the same person could through the API.
package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/app"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// latestProtocol is offered when the client asks for a version we do not know.
const latestProtocol = "2025-06-18"

var supportedProtocols = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// JSON-RPC error codes.
const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Server struct {
	cfg   *config.Config
	log   logger.Logger
	app   *app.App
	tools []*tool
	index map[string]*tool
}

func New(cfg *config.Config, log logger.Logger, a *app.App) *Server {
	s := &Server{cfg: cfg, log: log, app: a}
	s.tools = s.allTools()
	s.index = make(map[string]*tool, len(s.tools))
	for _, t := range s.tools {
		s.index[t.Name] = t
	}

	return s
}

// Handle is the single POST /mcp endpoint. It must sit behind Authenticate.
func (s *Server) Handle(c *gin.Context) {
	// DNS-rebinding guard: a browser page on another origin must not be able to
	// drive an agent endpoint on the user's machine. Non-browser clients send no Origin.
	if origin := c.GetHeader("Origin"); origin != "" && !s.originAllowed(origin) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "origin not allowed"}})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		writeJSON(c, http.StatusRequestEntityTooLarge, failure(nil, codeInvalidRequest, "request body too large"))
		return
	}

	body = bytes.TrimSpace(body)
	if len(body) > 0 && body[0] == '[' {
		s.handleBatch(c, body)
		return
	}

	resp := s.handleOne(c, body)
	if resp == nil { // a notification: acknowledged, nothing to say
		c.Status(http.StatusAccepted)
		return
	}

	writeJSON(c, http.StatusOK, resp)
}

func (s *Server) handleBatch(c *gin.Context, body []byte) {
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil || len(items) == 0 || len(items) > 20 {
		writeJSON(c, http.StatusBadRequest, failure(nil, codeInvalidRequest, "invalid batch"))
		return
	}

	out := make([]*response, 0, len(items))
	for _, item := range items {
		if r := s.handleOne(c, item); r != nil {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	writeJSON(c, http.StatusOK, out)
}

func (s *Server) handleOne(c *gin.Context, raw []byte) *response {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return failure(nil, codeParse, "parse error")
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		return failure(req.ID, codeInvalidRequest, "invalid request")
	}

	isNotification := len(req.ID) == 0
	var (
		result any
		rerr   *rpcError
	)

	switch req.Method {
	case "initialize":
		result, rerr = s.initialize(req.Params)
	case "ping":
		result = struct{}{}
	case "tools/list":
		result = s.listTools()
	case "tools/call":
		result, rerr = s.callTool(c, req.Params)
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		rerr = &rpcError{Code: codeMethodNotFound, Message: "method not found: " + req.Method}
	}

	if isNotification {
		return nil
	}
	if rerr != nil {
		return &response{JSONRPC: "2.0", ID: req.ID, Error: rerr}
	}

	return &response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) initialize(params json.RawMessage) (any, *rpcError) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params"}
		}
	}

	version := latestProtocol
	if slices.Contains(supportedProtocols, p.ProtocolVersion) {
		version = p.ProtocolVersion
	}

	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": config.AppName, "version": "1.0.0"},
		"instructions": "Tools to read and edit database diagrams written in DBML, organise them in nested folders, " +
			"share them with other people and keep version history. Everything is checked against the signed-in " +
			"person's access. Read the diagram (get_diagram) before changing it and pass its revision as " +
			"base_revision to update_diagram so you never overwrite someone else's edit.",
	}, nil
}

func (s *Server) originAllowed(origin string) bool {
	if len(s.cfg.CORSAllowedOrigins) == 0 {
		return !s.cfg.IsProduction()
	}

	return slices.Contains(s.cfg.CORSAllowedOrigins, origin)
}

// caller is who the tools act as.
type caller struct {
	UserID   string
	ReadOnly bool
}

func callerOf(c *gin.Context) caller {
	scope := middleware.ScopeOf(c)

	return caller{UserID: scope.UserID, ReadOnly: !scope.CanWrite()}
}

func failure(id json.RawMessage, code int, message string) *response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}

	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}

func writeJSON(c *gin.Context, status int, v any) {
	c.Header("Content-Type", "application/json")
	c.JSON(status, v)
}

// MethodNotAllowed answers GET /mcp: this server never streams to the client.
func MethodNotAllowed(c *gin.Context) {
	c.Header("Allow", "POST")
	c.AbortWithStatus(http.StatusMethodNotAllowed)
}

var errUnknownTool = errors.New("unknown tool")

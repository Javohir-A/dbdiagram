package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
	"gitlab.com/udevs/dbdiagram/backend/pkg/jwt"
)

const scopeKey = "scope"

// Scope is who is calling, as proven by the access token. It carries identity
// only: what the caller may do is looked up per request, never read from here.
type Scope struct {
	UserID string
	Family string
	// Token is set when the caller used a personal access token instead of a
	// login session. Empty means a full session.
	Token model.TokenScope
}

// ViaToken reports whether the caller is a personal access token.
func (s Scope) ViaToken() bool { return s.Token != "" }

// CanWrite is false only for a read-scoped token.
func (s Scope) CanWrite() bool { return s.Token != model.ScopeRead }

// ScopeOf returns the caller. Only valid behind Authenticate.
func ScopeOf(c *gin.Context) Scope {
	s, _ := c.Get(scopeKey)
	scope, _ := s.(Scope)

	return scope
}

func abort(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, dto.Error{Error: dto.ResponseError{Code: code, Message: message}})
}

// Authenticate requires a valid bearer credential: a login JWT, or a personal
// access token (recognised by its prefix). The client reacts to this 401 by
// refreshing once; every failure is the same answer, so a probe learns nothing
// about why a credential was refused.
func Authenticate(cfg *config.Config, tokens TokenAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || secret == "" {
			abort(c, http.StatusUnauthorized, apperr.CodeUnauthorized, "authentication required")
			return
		}

		if strings.HasPrefix(secret, core.TokenPrefix) {
			p, err := tokens.Authenticate(c.Request.Context(), secret)
			if err != nil {
				status, e := apperr.HTTPStatus(err)
				abort(c, status, e.Code, e.Message)
				return
			}

			// A read token may only make safe requests; the MCP endpoint decides per tool instead.
			if p.Scope == model.ScopeRead && !isSafe(c.Request.Method) && c.FullPath() != "/mcp" {
				abort(c, http.StatusForbidden, apperr.CodeForbidden, "this token is read-only")
				return
			}

			c.Set(scopeKey, Scope{UserID: p.UserID, Token: p.Scope})
			c.Next()

			return
		}

		claims, err := jwt.Parse(cfg.JWTSecret, secret, time.Now())
		if err != nil {
			abort(c, http.StatusUnauthorized, apperr.CodeUnauthorized, "invalid or expired token")
			return
		}

		c.Set(scopeKey, Scope{UserID: claims.UserID(), Family: claims.Family})
		c.Next()
	}
}

// TokenAuthenticator is what Authenticate needs from the accounts module.
type TokenAuthenticator interface {
	Authenticate(ctx context.Context, secret string) (*core.Principal, error)
}

func isSafe(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// RequireSession refuses personal access tokens: account management (profile,
// password, the tokens themselves) must never be reachable with a credential
// that could have been leaked from a script, or a token could mint its own
// replacement and outlive its revocation.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if ScopeOf(c).ViaToken() {
			abort(c, http.StatusForbidden, apperr.CodeForbidden, "this needs a signed-in session, not an access token")
			return
		}

		c.Next()
	}
}

// RateLimit allows `perMinute` requests per client IP per minute, in memory.
// That is exact for one instance; behind several, the limit is per instance.
// The client IP is trustworthy only because the engine trusts no proxy header
// unless TRUSTED_PROXIES says so.
func RateLimit(perMinute int) gin.HandlerFunc {
	type window struct {
		start time.Time
		count int
	}

	var (
		mu      sync.Mutex
		windows = map[string]*window{}
	)

	return func(c *gin.Context) {
		now := time.Now()
		ip := c.ClientIP()

		mu.Lock()
		w := windows[ip]
		if w == nil || now.Sub(w.start) >= time.Minute {
			// Drop expired entries now and then so the map cannot grow without bound.
			if len(windows) > 10000 {
				for k, v := range windows {
					if now.Sub(v.start) >= time.Minute {
						delete(windows, k)
					}
				}
			}
			w = &window{start: now}
			windows[ip] = w
		}
		w.count++
		over := w.count > perMinute
		retry := int(time.Minute.Seconds() - now.Sub(w.start).Seconds())
		mu.Unlock()

		if over {
			c.Header("Retry-After", itoa(max(retry, 1)))
			abort(c, http.StatusTooManyRequests, apperr.CodeRateLimited, "too many attempts, try again in a minute")
			return
		}

		c.Next()
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var b [12]byte

	i := len(b)
	for ; n > 0; n /= 10 {
		i--
		b[i] = byte('0' + n%10)
	}

	return string(b[i:])
}

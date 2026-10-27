// Package api builds the gin engine and runs it.
package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "gitlab.com/udevs/dbdiagram/backend/api/docs"
	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/handler"
	"gitlab.com/udevs/dbdiagram/backend/api/mcp"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/app"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type Options struct {
	Cfg *config.Config
	Log logger.Logger
	App *app.App
}

type Server struct {
	engine *gin.Engine
	log    logger.Logger
}

// @title dbdiagram API
// @version 1.0
// @description DBML diagrams in nested folders, shared with other people as owner, editor or viewer.
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Type "Bearer" and a space, then the access token from /v1/auth/login.
func New(opt *Options) *Server {
	if opt.Cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// The rate limiter keys on the client IP, so a forged X-Forwarded-For must
	// not be believed: only the proxies named in TRUSTED_PROXIES are.
	if err := engine.SetTrustedProxies(opt.Cfg.TrustedProxies); err != nil {
		opt.Log.Error("invalid TRUSTED_PROXIES, trusting none", logger.Error(err))
		_ = engine.SetTrustedProxies(nil)
	}
	engine.Use(
		middleware.RequestID(),
		gin.Logger(),
		gin.CustomRecovery(func(c *gin.Context, _ any) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, dto.Error{
				Error: dto.ResponseError{Code: apperr.CodeInternal, Message: "Internal Server Error"},
			})
		}),
	)

	// No cookies are used, so credentials stay off and a wildcard origin is
	// safe in development. Production must list its origins explicitly.
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Content-Type", "Authorization", middleware.RequestIDHeader}
	corsConfig.ExposeHeaders = []string{"Retry-After", middleware.RequestIDHeader}
	if len(opt.Cfg.CORSAllowedOrigins) > 0 {
		corsConfig.AllowOrigins = opt.Cfg.CORSAllowedOrigins
	} else {
		corsConfig.AllowAllOrigins = true
	}
	engine.Use(cors.New(corsConfig))

	engine.Use(
		middleware.Timeout(time.Duration(opt.Cfg.CtxTimeout)*time.Second),
		middleware.BodyLimit(config.MaxBodySize),
	)

	h := handler.New(&handler.Options{Cfg: opt.Cfg, Log: opt.Log, App: opt.App})
	// MCP: same credentials as the API (a login or a personal access token), no /v1 prefix.
	authenticate := middleware.Authenticate(opt.Cfg, opt.App.Core.Token)
	mcpServer := mcp.New(opt.Cfg, opt.Log, opt.App)
	engine.POST("/mcp", authenticate, mcpServer.Handle)
	engine.GET("/mcp", mcp.MethodNotAllowed)

	v1 := engine.Group("/v1")
	h.RegisterRoutes(v1, v1.Group("", authenticate), middleware.RateLimit(opt.Cfg.AuthRateLimitPerMin))

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("doc.json")))

	engine.NoRoute(func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusNotFound, dto.Error{
			Error: dto.ResponseError{Code: apperr.CodeNotFound, Message: "route not found"},
		})
	})

	return &Server{engine: engine, log: opt.Log}
}

// Handler exposes the engine for tests.
func (s *Server) Handler() http.Handler { return s.engine }

// Run serves until ctx is cancelled, then lets in-flight requests finish.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	s.log.Info("HTTP server is listening", logger.String("addr", addr))

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		s.log.Info("shutting down")

		return srv.Shutdown(shutdownCtx)
	}
}

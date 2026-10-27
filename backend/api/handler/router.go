package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
)

// RegisterRoutes wires every v1 endpoint. Keep one register<Group>Routes
// function per resource group so this file never grows into a wall of routes.
// There is no auth group yet: when it arrives, its middleware is passed in
// here and wraps the groups that need it.
// RegisterRoutes wires every v1 endpoint. Keep one register<Group>Routes
// function per resource group so this file never grows into a wall of routes.
// `public` carries no authentication; `private` is behind it. A new route goes
// into private unless it is deliberately open to anonymous callers.
func (h *Handler) RegisterRoutes(public, private *gin.RouterGroup, authLimit gin.HandlerFunc) {
	h.registerSystemRoutes(public)
	h.registerAuthRoutes(public, private, authLimit)
	h.registerSharedRoutes(public)

	h.registerDiagramRoutes(private)
	h.registerFolderRoutes(private)
}

func (h *Handler) registerSystemRoutes(r *gin.RouterGroup) {
	r.GET("/health", h.Health)
}

func (h *Handler) registerAuthRoutes(public, private *gin.RouterGroup, authLimit gin.HandlerFunc) {
	// Brute force and signup spam are the point of the limiter, so it sits on
	// exactly the routes that take a password or a refresh token.
	public.POST("/auth/register", authLimit, h.Register)
	public.POST("/auth/login", authLimit, h.Login)
	public.POST("/auth/refresh", authLimit, h.Refresh)
	public.POST("/auth/logout", h.Logout)

	private.GET("/me", h.GetMe)
	// Account management needs a real session: see middleware.RequireSession.
	private.PATCH("/me", middleware.RequireSession(), h.UpdateMe)
	private.PUT("/me/password", middleware.RequireSession(), authLimit, h.ChangePassword)
	private.GET("/me/tokens", middleware.RequireSession(), h.ListTokens)
	private.POST("/me/tokens", middleware.RequireSession(), h.CreateToken)
	private.DELETE("/me/tokens/:key", middleware.RequireSession(), h.RevokeToken)
}

func (h *Handler) registerDiagramRoutes(r *gin.RouterGroup) {
	r.GET("/diagrams", h.ListDiagrams)
	r.POST("/diagrams", h.CreateDiagram)
	r.GET("/diagrams/:key", h.GetDiagram)
	r.GET("/diagrams/:key/revision", h.GetDiagramRevision)
	r.PUT("/diagrams/:key", h.ReplaceDiagram)
	r.PATCH("/diagrams/:key", h.PatchDiagram)
	r.DELETE("/diagrams/:key", h.DeleteDiagram)
	r.PUT("/diagrams/:key/folder", h.SetDiagramFolder)
	r.PUT("/diagrams/:key/share", h.SetDiagramShare)

	r.GET("/diagrams/:key/versions", h.ListDiagramVersions)
	r.POST("/diagrams/:key/versions", h.CreateDiagramVersion)
	r.GET("/diagrams/:key/versions/:version", h.GetDiagramVersion)
	r.POST("/diagrams/:key/versions/:version/restore", h.RestoreDiagramVersion)
	r.GET("/diagrams/:key/members", h.ListDiagramMembers)
	r.PUT("/diagrams/:key/members", h.SetDiagramMember)
	r.DELETE("/diagrams/:key/members/:user", h.RemoveDiagramMember)
	r.DELETE("/diagrams/:key/versions/:version", h.DeleteDiagramVersion)
}

func (h *Handler) registerFolderRoutes(r *gin.RouterGroup) {
	r.GET("/folders", h.ListFolders)
	r.POST("/folders", h.CreateFolder)
	r.GET("/folders/:key", h.GetFolder)
	r.PUT("/folders/:key", h.ReplaceFolder)
	r.PATCH("/folders/:key", h.PatchFolder)
	r.PUT("/folders/:key/parent", h.SetFolderParent)
	r.DELETE("/folders/:key", h.DeleteFolder)
	r.GET("/folders/:key/members", h.ListFolderMembers)
	r.PUT("/folders/:key/members", h.SetFolderMember)
	r.DELETE("/folders/:key/members/:user", h.RemoveFolderMember)
}

// registerSharedRoutes is the only public read of a diagram.
func (h *Handler) registerSharedRoutes(r *gin.RouterGroup) {
	r.GET("/shared/:key", h.GetSharedDiagram)
}

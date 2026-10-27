// Package handler is the HTTP layer: bind, call one service method, respond.
package handler

import (
	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/app"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type Handler struct {
	cfg *config.Config
	log logger.Logger
	app *app.App
}

type Options struct {
	Cfg *config.Config
	Log logger.Logger
	App *app.App
}

func New(opt *Options) *Handler {
	return &Handler{cfg: opt.Cfg, log: opt.Log, app: opt.App}
}

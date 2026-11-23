package main

import (
	"log/slog"
	"net/http"

	"github.com/junkerderprovinz/shiplog/internal/config"
	"github.com/junkerderprovinz/shiplog/internal/gate"
	"github.com/junkerderprovinz/shiplog/internal/webui"
)

// front puts the sign-in before h when ShipLog answers on a port. On the
// plugin's socket Unraid's own login guards it.
func front(cfg config.Config, h http.Handler) (http.Handler, func() error, error) {
	if cfg.Socket != "" {
		return h, func() error { return nil }, nil
	}
	s := gate.Settings{Dir: cfg.DataDir, Key: cfg.AppKey, BaseURL: cfg.BaseURL, TrustProxy: cfg.TrustProxy}
	return gate.New(s, h, webui.Files(), slog.Default())
}

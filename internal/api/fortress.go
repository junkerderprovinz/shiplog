package api

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/junkerderprovinz/parleyport/kit/status"
)

// WithFortress serves the status contract from src. Only a request that came
// in on the engine's unix socket gets an answer, see Server.
func (a *API) WithFortress(src status.Source) *API {
	a.fortress = src
	return a
}

// Server is the HTTP server for the API. It marks every connection made on a
// unix socket, which only root and the WebGUI's proxy behind Unraid's login can
// open, so the status contract never answers on a TCP port.
func (a *API) Server() *http.Server {
	return &http.Server{
		Handler:           a.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			if _, ok := c.(*net.UnixConn); ok {
				return onSocket(ctx)
			}
			return ctx
		},
	}
}

type socketKey struct{}

func onSocket(ctx context.Context) context.Context {
	return context.WithValue(ctx, socketKey{}, true)
}

// fromSocket lets through what came in on the socket. The engine has no login
// of its own, so there is nothing else a caller could show.
func fromSocket(r *http.Request) error {
	if on, _ := r.Context().Value(socketKey{}).(bool); on {
		return nil
	}
	return status.ErrUnauthenticated
}

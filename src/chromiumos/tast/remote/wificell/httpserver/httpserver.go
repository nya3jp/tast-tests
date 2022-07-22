// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package httpserver provides the utils to run an httpserver on the remote router.
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

// Paths in chroot.
const (
	logPath     = "/tmp/httpServer.log"
	redirectURL = "http://www.foo.com"
)

// Httpserver handles http server and its listening port.
type Httpserver struct {
	// port is the port that the HTTP server will listen and serve on.
	port   string
	server *http.Server
}

// Handler creates the object to handle the response for the HTTP server.
type Handler struct{}

// ServeHTTP will have the HTTP server respond with 302 redirects and a redirect URL.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	http.Redirect(rw, req, redirectURL, http.StatusFound)
}

// New creates a new httpserver object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object. The
// httpserver will only respond with 302 redirects. Port will be the port the
// HTTP server listens and serves on.
func New(port string) *Httpserver {
	return &Httpserver{port: port}
}

// StartOnRouter starts the HTTP server in a separate process and on a remote Router.
// The HTTP server listens on any IPv4 and IPv6 address within the namespace.
func (h *Httpserver) StartOnRouter(ctx context.Context) (retErr error) {
	h.server = &http.Server{Addr: fmt.Sprintf(":%v", h.port), Handler: &Handler{}}

	errChannel := make(chan error)
	go func() {
		ln, err := net.Listen("tcp", h.server.Addr)
		if err != nil {
			errChannel <- err
			return
		}
		errChannel <- nil
		h.server.Serve(ln)
	}()

	return <-errChannel
}

// Stop terminates the process running the HTTP server.
func (h *Httpserver) Stop(ctx context.Context) error {
	h.server.Shutdown(ctx)
	return nil
}

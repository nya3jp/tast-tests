// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package httpserver provides the utils to run an httpserver inside a
// virtualnet.Env.
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
)

// Paths in chroot.
const (
	logPath     = "/tmp/httpServer.log"
	redirectURL = "http://www.foo.com"
)

type httpserver struct {
	// port is the port that the HTTP server will listen and serve on.
	port   string
	server *http.Server
	env    *env.Env
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
func New(port string) *httpserver {
	return &httpserver{port: port}
}

// Start starts the HTTP server in a separate process. The HTTP server listens on
// any IPv4 and IPv6 address within the namespace.
func (h *httpserver) Start(ctx context.Context, env *env.Env) (retErr error) {
	h.env = env
	h.server = &http.Server{Addr: fmt.Sprintf(":%v", h.port), Handler: &Handler{}}
	errChannel := make(chan error)
	go func() {
		cleanup, err := h.env.EnterNetNS(ctx)
		if err != nil {
			errChannel <- errors.Wrapf(err, "failed to enter the associated netns %s", h.env.NetNSName)
			return
		}
		defer cleanup()
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
func (h *httpserver) Stop(ctx context.Context) error {
	h.server.Shutdown(ctx)
	return nil
}

// WriteLogs writes logs into |f|.
func (h *httpserver) WriteLogs(ctx context.Context, f *os.File) error {
	return h.env.ReadAndWriteLogIfExists(h.env.ChrootPath(logPath), f)
}

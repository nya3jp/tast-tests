// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package httpserver provides the utils to run an httpServer inside a
// virtualnet.Env.
package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network/virtualnet/certs"
	"chromiumos/tast/local/network/virtualnet/env"
	"chromiumos/tast/testing"
)

// Path in chroot.
const (
	logPath = "/tmp/httpServer.log"
)

// Path in root to bind mount to for cert validation
const (
	sslCrtPath = "/etc/ssl/certs"
)

type httpServer struct {
	// port is the port that the HTTP server will listen and serve on.
	port string
	// serveTLS is true if the server is using HTTPS. If false, the server is using HTTP.
	serveTLS bool
	handle   func(rw http.ResponseWriter, req *http.Request)
	server   *http.Server
	env      *env.Env
}

// Handler creates the object to handle the response for the HTTP server.
type Handler struct {
	handle func(rw http.ResponseWriter, req *http.Request)
}

// ServeHTTP will have the HTTP server respond to requests with |handle|.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.handle(rw, req)
}

// New creates a new httpServer object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object. The
// httpServer will only respond with |handle|. |port| will be the port the
// HTTP server listens and serves on. |serveTLS| is set to true if the server
// serves HTTPS. If false, the server will serve HTTP.
func New(port string, handle func(rw http.ResponseWriter, req *http.Request), serveTLS bool) *httpServer {
	return &httpServer{port: port, handle: handle, serveTLS: serveTLS}
}

// Start starts the HTTP server in a separate process. The HTTP server listens on
// any IPv4 and IPv6 address within the namespace. If |serveTLS| is true, the server
// will server HTTPS.
func (h *httpServer) Start(ctx context.Context, env *env.Env) (retErr error) {
	h.env = env
	handler := &Handler{handle: h.handle}
	h.server = &http.Server{Addr: fmt.Sprintf(":%v", h.port), Handler: handler}

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
		if h.serveTLS {
			httpsCerts := certs.New(sslCrtPath, certificate.TestCert3())
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
			defer cancel()
			cleanupCerts, err := httpsCerts.InstallTestCerts(ctx)
			if err != nil {
				cleanupCerts(ctx)
				errChannel <- err
				return
			}
			defer cleanupCerts(cleanupCtx)
			errChannel <- nil
			if err := h.server.ServeTLS(ln, httpsCerts.GetTestServerCertFilePath(), httpsCerts.GetTestServerKeyFilePath()); err != http.ErrServerClosed {
				testing.ContextLog(ctx, "ServeTLS failed to start with err: ", err)
			}
		} else {
			errChannel <- nil
			h.server.Serve(ln)
		}
	}()

	return <-errChannel
}

// Stop terminates the process running the HTTP server.
func (h *httpServer) Stop(ctx context.Context) error {
	h.server.Shutdown(ctx)
	return nil
}

// WriteLogs writes logs into |f|.
func (h *httpServer) WriteLogs(ctx context.Context, f *os.File) error {
	return h.env.ReadAndWriteLogIfExists(h.env.ChrootPath(logPath), f)
}

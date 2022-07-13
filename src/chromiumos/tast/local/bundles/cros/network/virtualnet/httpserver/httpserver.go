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

	"chromiumos/tast/common/crypto/certificate"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/virtualnet/env"
)

// Paths in chroot.
const (
	keyPath = "/tmp/test.key"
	crtPath = "/tmp/test.crt"
	logPath = "/tmp/httpServer.log"
)

type httpserver struct {
	// port is the port that the HTTP server will listen and serve on.
	port string
	// serverCredentials is set if the server is https.
	serverCredentials *certificate.Credential
	handle            func(rw http.ResponseWriter, req *http.Request)
	server            *http.Server
	env               *env.Env
}

// Handler creates the object to handle the response for the HTTP server.
type Handler struct {
	handle func(rw http.ResponseWriter, req *http.Request)
}

// ServeHTTP will have the HTTP server respond to requests with |handle|.
func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.handle(rw, req)
}

// New creates a new httpserver object. The returned object can be passed to
// Env.StartServer(), its lifetime will be managed by the Env object. The
// httpserver will only respond with |handle|. |port| will be the port the
// HTTP server listens and serves on. |serverCredentials| will contain the credentials
// for an HTTPS server. If this is nil, the server only listens for HTTP.
func New(port string, serverCredentials *certificate.Credential, handle func(rw http.ResponseWriter, req *http.Request)) *httpserver {
	return &httpserver{port: port, serverCredentials: serverCredentials, handle: handle}
}

// Start starts the HTTP server in a separate process. The HTTP server listens on
// any IPv4 and IPv6 address within the namespace. If |serverCredentials| is set,
// the server started will be HTTPS.
func (h *httpserver) Start(ctx context.Context, env *env.Env) (retErr error) {
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
		if h.serverCredentials != nil {
			if err := os.WriteFile(crtPath, []byte(h.serverCredentials.Cert), 0644); err != nil {
				errChannel <- err
				return
			}
			if err := os.WriteFile(keyPath, []byte(h.serverCredentials.PrivateKey), 0644); err != nil {
				errChannel <- err
				return
			}

			errChannel <- nil
			err = h.server.ServeTLS(ln, crtPath, keyPath)
		} else {
			errChannel <- nil
			h.server.Serve(ln)
		}
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

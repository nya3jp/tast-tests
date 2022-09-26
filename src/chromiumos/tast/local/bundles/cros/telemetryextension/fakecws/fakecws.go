// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package fakecws

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type httpHandler struct {
	logger *testing.Logger
	selfAddr string
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h.logger != nil {
		h.logger.Printf("Fake Chrome Web Store received HTTP request for %q", req.URL.Path)
	}

	xml := fmt.Sprintf(`
<gupdate xmlns="http://www.google.com/update2/response" protocol="2.0">
	<app appid="gogonhoemckpdpadfnjnpgbjpbjnodgc">
		<updatecheck codebase="http://%s/extension.crx" version="1.4.0"/>
	</app>
</gupdate>
`, h.selfAddr)

	h.logger.Printf(xml)

	w.Write([]byte(xml))
}

// Server is a fake Chrome Web Store.
type Server struct {
	httpServer *http.Server
	handler    httpHandler
	port int
}

// NewServer creates a new fake Chrome Web Store server.
func NewServer(ctx context.Context) (*Server, error) {
	logger, ok := testing.ContextLogger(ctx)
	if !ok {
		// To allow golang testing
		logger = nil
	}

	srv := Server{
		httpServer: &http.Server{},
		handler: httpHandler{
			logger: logger,
		},
	}

	srv.httpServer.Handler = &srv.handler

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create listener for http server")
	}

	port := listener.Addr().(*net.TCPAddr).Port
	srv.httpServer.Addr = fmt.Sprintf("127.0.0.1:%d", port)
	srv.port = port
	srv.handler.selfAddr = srv.httpServer.Addr

	go func() {
		if err := srv.httpServer.Serve(listener); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "Chrome Web Store HTTP server failed: ", err)
		}
	}()

	return &srv, nil
}

// URL returns http server URL.
func (s *Server) URL() string {
	// return fmt.Sprintf("http://%s/service/update2/crx", s.httpServer.Addr)
	return fmt.Sprintf("http://localhost:%d/service/update2/crx", s.port)
}

// Stop shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown HTTP server")
	}

	return nil
}

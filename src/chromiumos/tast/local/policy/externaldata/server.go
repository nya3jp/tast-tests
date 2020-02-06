// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package externaldata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

type httpHandler struct {
	logger *testing.Logger

	policies map[string][]byte
	mu       sync.Mutex // Protects policies.
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if h.logger != nil {
		h.logger.Printf("ExternalDataServer received HTTP request for %q", req.URL.Path)
	}

	h.mu.Lock()
	policy, ok := h.policies[req.URL.Path]
	h.mu.Unlock()

	if !ok {
		if h.logger != nil {
			h.logger.Print("Failed to find: ", req.URL.Path)
		}
		http.Error(w, http.StatusText(404), 404)
		return
	}

	w.Write(policy)
}

// Server is a http server that helps serve data for policies that load their data from an external source.
type Server struct {
	httpServer *http.Server
	handler    httpHandler
}

// NewServer creates a new external data server.
func NewServer(ctx context.Context) (*Server, error) {
	logger, ok := testing.ContextLogger(ctx)
	if !ok {
		// To allow golang testing
		logger = nil
	}

	srv := Server{
		httpServer: &http.Server{},
		handler: httpHandler{
			policies: make(map[string][]byte),
			logger:   logger,
		},
	}

	srv.httpServer.Handler = &srv.handler

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create listener for http server")
	}

	port := listener.Addr().(*net.TCPAddr).Port
	srv.httpServer.Addr = fmt.Sprintf("127.0.0.1:%d", port)

	go func() {
		if err := srv.httpServer.Serve(listener); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "ExternalDataServer HTTP server failed: ", err)
		}
	}()

	return &srv, nil
}

// ServePolicyData starts serving content and returns the URL and hash to be set in the policy.
func (s *Server) ServePolicyData(data []byte) (address, hash string) {
	sum := sha256.Sum256(data)
	hash = hex.EncodeToString(sum[:])

	// Using len(s.policies) as a prefix ensures a unique URL.
	// Part of the hash is used to lengthen the path to be more realistic.
	path := fmt.Sprintf("/%d-%s", len(s.handler.policies), hash[:5])

	s.handler.mu.Lock()
	s.handler.policies[path] = append([]byte(nil), data...)
	s.handler.mu.Unlock()

	url := url.URL{
		Host:   s.httpServer.Addr,
		Path:   path,
		Scheme: "http",
	}

	return url.String(), hash
}

// Stop shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown HTTP server")
	}

	return nil
}

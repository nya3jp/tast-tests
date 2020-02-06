// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package externaldata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Server is a http server that helps serve data for policies that load their data from an external source.
type Server struct {
	httpServer *http.Server
	port       int
	domain     string
	logger     *testing.Logger

	policies map[string][]byte
	mu       sync.Mutex // Protects policies.
}

// NewExternalDataServer creates a new policy server on the specified port.
// domain is used to generate correct URLs.
func NewExternalDataServer(ctx context.Context, domain string, port int) (*Server, error) {
	logger, ok := testing.ContextLogger(ctx)
	if !ok {
		// To allow golang testing
		logger = nil
	}

	srv := Server{
		httpServer: &http.Server{Addr: fmt.Sprintf(":%d", port)},
		port:       port,
		domain:     domain,
		policies:   make(map[string][]byte),
		logger:     logger,
	}

	srv.httpServer.Handler = &srv

	go func() {
		if err := srv.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "ExternalDataServer HTTP server failed: ", err)
		}
	}()

	return &srv, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if s.logger != nil {
		s.logger.Printf("ExternalDataServer received HTTP request for %q", req.URL.Path)
	}

	s.mu.Lock()
	policy, ok := s.policies[req.URL.Path]
	s.mu.Unlock()

	if !ok {
		if s.logger != nil {
			s.logger.Print("Failed to find: ", req.URL.Path)
		}
		http.Error(w, http.StatusText(404), 404)
		return
	}

	w.Write(policy)
}

// ServePolicyData starts serving content and returns the URL and hash to be set in the policy.
func (s *Server) ServePolicyData(data []byte) (string, string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	path := fmt.Sprintf("/%d-%s", len(s.policies), hash[:5])

	s.mu.Lock()
	s.policies[path] = append([]byte(nil), data...)
	s.mu.Unlock()

	url := url.URL{
		Host:   fmt.Sprintf("%s:%d", s.domain, s.port),
		Path:   path,
		Scheme: "http",
	}

	return url.String(), hash, nil
}

// Stop shuts down the server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown HTTP server")
	}

	return nil
}

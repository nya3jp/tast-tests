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
	"sync"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// Server is a http server that helps serve data for policies that load their data from an external source.
type Server struct {
	httpServer *http.Server
	port       int
	domain     string
	policies   map[string]*policyData
	logger     *testing.Logger
	mux        sync.Mutex
}

type policyData struct {
	data []byte
}

// NewExternalDataServer creates a new policy server on the specified port.
// domain is used to generate correct URLs.
func NewExternalDataServer(ctx context.Context, domain string, port int) (*Server, error) {
	logger, ok := testing.ContextLogger(ctx)
	if !ok {
		return nil, errors.New("failed to get a logger")
	}

	srv := Server{
		httpServer: &http.Server{Addr: fmt.Sprintf(":%d", port)},
		port:       port,
		domain:     domain,
		policies:   make(map[string]*policyData),
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

func (eds *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	eds.mux.Lock()
	defer eds.mux.Unlock()

	eds.logger.Printf("ExternalDataServer received HTTP request for %q", req.URL.Path)

	if len(req.URL.Path) < 2 {
		eds.logger.Printf("Url too short %q", req.URL.Path)
		http.Error(w, http.StatusText(404), 404)
		return
	}

	policy, ok := eds.policies[req.URL.Path[1:]]

	if !ok {
		eds.logger.Print("Failed to find: ", req.URL.Path)
		http.Error(w, http.StatusText(404), 404)
		return
	}

	w.Write(policy.data)
}

// ServePolicyData starts serving content and returns the URL and hash to be set in the policy.
func (eds *Server) ServePolicyData(data []byte) (string, string, error) {
	eds.mux.Lock()
	defer eds.mux.Unlock()

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	path := fmt.Sprintf("%d-%s", len(eds.policies), hash[:5])

	eds.policies[path] = &policyData{
		data: append([]byte(nil), data...),
	}

	url := fmt.Sprintf("http://%s:%d/%s", eds.domain, eds.port, path)

	return url, hash, nil
}

// Stop shuts down the server.
func (eds *Server) Stop(ctx context.Context) error {
	if err := eds.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown HTTP server")
	}

	return nil
}

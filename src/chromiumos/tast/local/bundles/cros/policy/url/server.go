// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package url

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

// PolicyServer is a http server that helps serve data for policies that load their data from a URL.
type PolicyServer struct {
	httpServer *http.Server
	port       int
	domain     string
	policies   map[string]*policyData
	counter    int
	logger     *testing.Logger
}

type policyData struct {
	data   []byte
	sha256 [32]byte
}

func (ps *PolicyServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	policy, ok := ps.policies[req.URL.Path[1:]]

	if !ok {
		ps.logger.Print("Failed to find: ", req.URL.Path)
		http.Error(w, http.StatusText(404), 404)
		return
	}

	w.Write(policy.data)
}

// NewPolicyServer creates a new policy server on the specified port.
// domain is used to generate correct URLs.
func NewPolicyServer(ctx context.Context, port int, domain string) (*PolicyServer, error) {
	logger, ok := testing.ContextLogger(ctx)
	if !ok {
		return nil, errors.New("failed to get a logger")
	}

	srv := PolicyServer{
		httpServer: &http.Server{Addr: fmt.Sprintf(":%d", port)},
		port:       port,
		domain:     domain,
		policies:   make(map[string]*policyData),
		counter:    1,
		logger:     logger,
	}

	srv.httpServer.Handler = &srv

	go func() {
		if err := srv.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			testing.ContextLog(ctx, "PolicyServer HTTP server failed: ", err)
		}
	}()

	return &srv, nil
}

// ServePolicy starts serving content and returns the URL and hash to be used in the policy.
func (ps *PolicyServer) ServePolicy(data []byte) (string, string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	path := fmt.Sprintf("%d-%s", ps.counter, hash[:5])
	ps.counter++

	ps.policies[path] = &policyData{
		data:   data,
		sha256: sum,
	}

	url := fmt.Sprintf("http://%s:%d/%s", ps.domain, ps.port, path)

	return hash, url, nil
}

// Stop shuts down the server
func (ps *PolicyServer) Stop(ctx context.Context) error {
	if err := ps.httpServer.Shutdown(ctx); err != nil {
		return errors.Wrap(err, "failed to shutdown HTTP server")
	}

	return nil
}

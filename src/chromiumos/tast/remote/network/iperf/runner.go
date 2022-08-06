// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	commandTimeoutMargin = 30 * time.Second
)

// Runner manages running Iperf on a client/server pair.
type Runner struct {
	config *Config
	client Client
	server Server
}

// NewRunner creates a new Iperf runner.
func NewRunner(config *Config, client Client, server Server) *Runner {
	return &Runner{
		config: config,
		client: client,
		server: server,
	}
}

// Run attempts to run Iperf with the given number of retries and returns the result.
func (r *Runner) Run(ctx context.Context, retryCount int) (*Result, error) {
	for count := 0; count < retryCount; count++ {
		result, err := r.runSingle(ctx)
		if err == nil {
			return result, nil
		}
		testing.ContextLogf(ctx, "Failed to run Iperf attempt %d of %d: %v", count+1, retryCount, err)
	}

	return nil, errors.New("failed to run Iperf")
}

// runSingle attempts to run Iperf a single time.
func (r *Runner) runSingle(ctx context.Context) (*Result, error) {
	if err := r.server.Stop(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to stop Iperf server, err: ", err)
	}

	err := r.server.Start(ctx, r.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Iperf server")
	}
	defer r.server.Stop(ctx)

	result, err := r.client.Start(ctx, r.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run Iperf client")
	}

	if r.config.FetchServerResults {
		result, err = r.server.FetchResult(ctx, r.config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch results from Iperf server")
		}
	}

	return result, nil
}

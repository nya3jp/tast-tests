// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/remote/network/iperf"
	"chromiumos/tast/testing"
)

// CallboxIperfServer represents a callbox iperf server.
type CallboxIperfServer struct {
	client  *manager.CallboxManagerClient
	callbox string
	poller  *serverResultPoller
}

// NewCallboxIperfServer creates a CallboxServer for the given callbox.
func NewCallboxIperfServer(callbox string, client *manager.CallboxManagerClient) (*CallboxIperfServer, error) {
	return &CallboxIperfServer{
		client:  client,
		callbox: callbox,
		poller:  newServerResultPoller(callbox, client),
	}, nil
}

// Start starts an iperf server session on the callbox with the given configuration.
func (c *CallboxIperfServer) Start(ctx context.Context, cfg *iperf.Config) error {
	if cfg.WindowSize < iperf.KB {
		return errors.New("minimum allowed callbox window size is 1 KByte")
	}

	// stop any Iperf instances before attempting to configure
	if err := c.Stop(ctx); err != nil {
		return errors.Wrap(err, "failed to stop Iperf session on the callbox")
	}

	serverCtx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.ConfigureIperf(serverCtx, newServerRequest(c, cfg)); err != nil {
		return errors.Wrap(err, "failed to configure Iperf server on the callbox")
	}

	serverCtx, cancel = context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StartIperf(serverCtx, &manager.StartIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to start Iperf server on the callbox")
	}

	if cfg.FetchServerResults {
		if err := c.poller.start(ctx, cfg); err != nil {
			return errors.Wrap(err, "failed to start Iperf server results poller")
		}
	}

	return nil
}

// Close releases any additional resources held open by the server.
func (c *CallboxIperfServer) Close(ctx context.Context) {
	stopCtx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.CloseIperf(stopCtx, &manager.CloseIperfRequestBody{Callbox: c.callbox}); err != nil {
		testing.ContextLog(ctx, "Failed to close Iperf, err: ", err)
	}

	stopCtx, cancel = context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.poller.stop(stopCtx); err != nil {
		testing.ContextLog(ctx, "Failed to stop iperf server results poller, err: ", err)
	}
}

// Stop terminates any Iperf servers running on the remote machine.
func (c *CallboxIperfServer) Stop(ctx context.Context) error {
	stopCtx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	var allErrors error
	if err := c.client.StopIperf(stopCtx, &manager.StopIperfRequestBody{Callbox: c.callbox}); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to stop iperf on the callbox: %v", err) // NOLINT
	}

	stopCtx, cancel = context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.poller.stop(stopCtx); err != nil {
		allErrors = errors.Wrapf(allErrors, "failed to stop iperf server results poller: %v", err) // NOLINT
	}

	return allErrors
}

// FetchResult fetches the most recently available result from the callbox server.
func (c *CallboxIperfServer) FetchResult(ctx context.Context, config *iperf.Config) (*iperf.Result, error) {
	results, err := c.poller.fetchResult(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch results from poller ")
	}

	return results, nil
}

func newServerRequest(c *CallboxIperfServer, config *iperf.Config) *manager.ConfigureIperfRequestBody {
	return &manager.ConfigureIperfRequestBody{
		Callbox: c.callbox,
		// Pad server test time since the callbox will automatically stop the server after that amount of time
		Time: int((config.TestTime + testTimeMargin) / time.Second),
		Servers: []manager.IperfServerConfig{
			manager.IperfServerConfig{
				Protocol:   string(config.Protocol),
				Port:       config.Port,
				WindowSize: int64(config.WindowSize / iperf.KB),
			},
		},
	}
}

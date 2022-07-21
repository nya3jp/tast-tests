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
}

// NewCallboxIperfServer creates a CallboxServer for the given callbox.
func NewCallboxIperfServer(callbox string, client *manager.CallboxManagerClient) (*CallboxIperfServer, error) {
	return &CallboxIperfServer{
		client:  client,
		callbox: callbox,
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

	return nil
}

// Close releases any additional resources held open by the server.
func (c *CallboxIperfServer) Close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.CloseIperf(ctx, &manager.CloseIperfRequestBody{Callbox: c.callbox}); err != nil {
		testing.ContextLog(ctx, "Failed to close Iperf, err: ", err)
	}
}

// Stop terminates any Iperf servers running on the remote machine.
func (c *CallboxIperfServer) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StopIperf(ctx, &manager.StopIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to stop Iperf on the callbox")
	}

	return nil
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

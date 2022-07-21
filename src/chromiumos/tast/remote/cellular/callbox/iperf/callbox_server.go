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

// CallboxServer represents a callbox iperf server.
type CallboxServer struct {
	client  *manager.CallboxManagerClient
	callbox string
}

// NewCallboxServer creates a CallboxServer for the given callbox.
func NewCallboxServer(callbox string, client *manager.CallboxManagerClient) (*CallboxServer, error) {
	return &CallboxServer{
		client:  client,
		callbox: callbox,
	}, nil
}

// Start starts an iperf server session on the callbox with the given configuration.
func (c *CallboxServer) Start(ctx context.Context, cfg *iperf.Config) error {
	if cfg.WindowSize < iperf.KB {
		return errors.New("minimum allowed callbox window size is 1 KByte")
	}

	// stop any Iperf instances before attempting to configure
	c.Stop(ctx)

	serverCtx, cancel := context.WithTimeout(ctx, 2*commandTimeoutMargin)
	defer cancel()

	if err := c.client.ConfigureIperf(serverCtx, newServerRequest(c, cfg)); err != nil {
		return errors.Wrap(err, "failed to configure Iperf server on the callbox")
	}

	if err := c.client.StartIperf(serverCtx, &manager.StartIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to start Iperf server on the callbox")
	}

	return nil
}

// Close releases any additional resources held open by the server.
func (c *CallboxServer) Close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.CloseIperf(ctx, &manager.CloseIperfRequestBody{Callbox: c.callbox}); err != nil {
		testing.ContextLog(ctx, "Failed to close Iperf, err: ", err)
	}
}

// Stop terminates any Iperf servers running on the remote machine.
func (c *CallboxServer) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StopIperf(ctx, &manager.StopIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to stop Iperf on the callbox")
	}

	return nil
}

func newServerRequest(c *CallboxServer, config *iperf.Config) *manager.ConfigureIperfRequestBody {
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

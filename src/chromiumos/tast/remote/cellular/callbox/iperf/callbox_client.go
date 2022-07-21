// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/cellular/callbox/manager"
	"chromiumos/tast/remote/network/iperf"
	"chromiumos/tast/testing"
)

// CallboxClient represents a callbox iperf client.
type CallboxClient struct {
	client  *manager.CallboxManagerClient
	callbox string
}

// NewCallboxClient creates a CallboxClient for the given callbox.
func NewCallboxClient(callbox string, client *manager.CallboxManagerClient) (*CallboxClient, error) {
	return &CallboxClient{
		client:  client,
		callbox: callbox,
	}, nil
}

// Start starts an iperf client session on the callbox with the given configuration.
func (c *CallboxClient) Start(ctx context.Context, config *iperf.Config) (*iperf.Result, error) {
	if config.Protocol == iperf.ProtocolUDP && config.PortCount != 1 {
		return nil, errors.New("iperf callbox client does not support parallel connections over UDP")
	}
	if config.Bidirectional {
		return nil, errors.New("iperf callbox client does not support bidirectional tests")
	}
	if config.WindowSize < iperf.KB {
		return nil, errors.New("minimum allowed callbox window size is 1 KByte")
	}

	// stop any Iperf instances before attempting to configure
	c.Stop(ctx)

	ctx, cancel := context.WithTimeout(ctx, config.TestTime+testTimeMargin+4*commandTimeoutMargin)
	defer cancel()

	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 2*commandTimeoutMargin)
	defer cancel()

	setupCtx, cancel := context.WithTimeout(ctx, 2*commandTimeoutMargin)
	defer cancel()

	if err := c.client.ConfigureIperf(setupCtx, newClientRequest(c, config)); err != nil {
		return nil, errors.Wrap(err, "failed to configure Iperf client on the callbox")
	}

	if err := c.client.StartIperf(setupCtx, &manager.StartIperfRequestBody{Callbox: c.callbox}); err != nil {
		return nil, errors.Wrap(err, "failed to start Iperf client on the callbox")
	}
	defer c.Stop(cleanupCtx)

	// callbox results are updated every 1 second, poll every 0.5 seconds and ignore duplicate results
	seen := make(map[int]bool)
	endTime := time.Now().Add(config.TestTime + testTimeMargin)
	var totalThroughput float64
	for time.Now().Before(endTime) {
		result, err := c.client.FetchIperfResult(ctx, &manager.FetchIperfResultRequestBody{Callbox: c.callbox})
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch results from callbox")
		}

		clientResult := result.Clients[0]
		if clientResult != nil && !seen[clientResult.ID] {
			seen[clientResult.ID] = true
			totalThroughput += clientResult.Throughput
		}

		if err := testing.Sleep(ctx, time.Second/2); err != nil {
			return nil, errors.Wrap(err, "failed to sleep while polling for results")
		}
	}

	sampleCount := len(seen)
	if sampleCount == 0 {
		return nil, errors.New("failed to get results from callbox, no useable data found")
	}

	return &iperf.Result{
		Duration:   time.Duration(sampleCount) * time.Second,
		Throughput: iperf.BitRate(totalThroughput / float64(sampleCount)),
	}, nil
}

// Close releases any additional resources held open by the server.
func (c *CallboxClient) Close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.CloseIperf(ctx, &manager.CloseIperfRequestBody{Callbox: c.callbox}); err != nil {
		testing.ContextLog(ctx, "Failed to close Iperf on the callbox, err: ", err)
	}
}

// Stop terminates any Iperf clients running on the remote machine.
func (c *CallboxClient) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StopIperf(ctx, &manager.StopIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to stop Iperf on the callbox")
	}

	return nil
}

func newClientRequest(c *CallboxClient, config *iperf.Config) *manager.ConfigureIperfRequestBody {
	return &manager.ConfigureIperfRequestBody{
		Callbox:    c.callbox,
		Time:       int(config.TestTime / time.Second),
		PacketSize: 1500,
		Clients: []manager.IperfClientConfig{
			manager.IperfClientConfig{
				IP:                  config.ServerIP,
				Protocol:            string(config.Protocol),
				Port:                config.Port,
				WindowSize:          int64(config.WindowSize / iperf.KB),
				ParallelConnections: config.PortCount,
				MaxBitRate:          float64(config.MaxBandwidth),
			},
		},
	}
}

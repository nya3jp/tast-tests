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

// CallboxIperfClient represents a callbox iperf client.
type CallboxIperfClient struct {
	client  *manager.CallboxManagerClient
	callbox string
}

// NewCallboxIperfClient creates a CallboxClient for the given callbox.
func NewCallboxIperfClient(callbox string, client *manager.CallboxManagerClient) (*CallboxIperfClient, error) {
	return &CallboxIperfClient{
		client:  client,
		callbox: callbox,
	}, nil
}

// Start starts an iperf client session on the callbox with the given configuration.
func (c *CallboxIperfClient) Start(ctx context.Context, config *iperf.Config) (*iperf.Result, error) {
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
	if err := c.Stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop Iperf session on the callbox")
	}

	ctx, cancel := context.WithTimeout(ctx, config.TestTime+testTimeMargin+4*commandTimeoutMargin)
	defer cancel()

	cleanupCtx := ctx
	ctx, cancel = ctxutil.Shorten(ctx, 2*commandTimeoutMargin)
	defer cancel()

	setupCtx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.ConfigureIperf(setupCtx, newClientRequest(c, config)); err != nil {
		return nil, errors.Wrap(err, "failed to configure Iperf client on the callbox")
	}

	setupCtx, cancel = context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StartIperf(setupCtx, &manager.StartIperfRequestBody{Callbox: c.callbox}); err != nil {
		return nil, errors.Wrap(err, "failed to start Iperf client on the callbox")
	}
	defer c.Stop(cleanupCtx)

	// callbox results are updated every 1 second, poll every 0.5 seconds and ignore duplicate results
	expectedCount := int(config.TestTime / time.Second)
	seen := make(map[int]bool)
	endTime := time.Now().Add(config.TestTime + testTimeMargin)
	var averageThroughput float64
	for time.Now().Before(endTime) {
		result, err := c.client.FetchIperfResult(ctx, &manager.FetchIperfResultRequestBody{Callbox: c.callbox})
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch results from callbox")
		}

		if result == nil || result.Clients == nil || len(result.Clients) < 1 {
			return nil, errors.New("failed to fetch client result from callbox, result was nil")
		}

		clientResult := result.Clients[0]
		if clientResult != nil && !seen[clientResult.ID] {
			seen[clientResult.ID] = true
			averageThroughput += clientResult.Throughput / float64(expectedCount)
		}

		// The callbox client automatically includes an additional padding which may result in additional samples
		if len(seen) == expectedCount {
			break
		}

		if err := testing.Sleep(ctx, time.Second/2); err != nil {
			return nil, errors.Wrap(err, "failed to sleep while polling for results")
		}
	}

	// Wait for endTime even if polling is complete as attempting to stop callbox Iperf client early may prevent server from terminating.
	if err := testing.Sleep(ctx, endTime.Sub(time.Now())); err != nil {
		return nil, errors.Wrap(err, "failed to sleep while waiting for server to stop")
	}

	sampleCount := len(seen)
	if sampleCount != expectedCount {
		return nil, errors.Errorf("failed to get results from callbox, missing data: got %d lines expected %d", sampleCount, expectedCount)
	}

	return &iperf.Result{
		Duration:   config.TestTime,
		Throughput: iperf.BitRate(averageThroughput),
	}, nil
}

// Close releases any additional resources held open by the server.
func (c *CallboxIperfClient) Close(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.CloseIperf(ctx, &manager.CloseIperfRequestBody{Callbox: c.callbox}); err != nil {
		testing.ContextLog(ctx, "Failed to close Iperf on the callbox, err: ", err)
	}
}

// Stop terminates any Iperf clients running on the remote machine.
func (c *CallboxIperfClient) Stop(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, commandTimeoutMargin)
	defer cancel()

	if err := c.client.StopIperf(ctx, &manager.StopIperfRequestBody{Callbox: c.callbox}); err != nil {
		return errors.Wrap(err, "failed to stop Iperf on the callbox")
	}

	return nil
}

func newClientRequest(c *CallboxIperfClient, config *iperf.Config) *manager.ConfigureIperfRequestBody {
	return &manager.ConfigureIperfRequestBody{
		Callbox: c.callbox,
		Time:    int(config.TestTime / time.Second),
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

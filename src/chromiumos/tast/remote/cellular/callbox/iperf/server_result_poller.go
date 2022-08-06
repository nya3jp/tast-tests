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

// serverResultPoller is responsible for starting/stopping callbox server results polling sessions in the background.
//
// Note: while the polling session runs the results in the background, the resultsPoller struct itself is not thread safe i.e. multiple start/stop calls cannot be made in parallel.
type serverResultPoller struct {
	client         *manager.CallboxManagerClient
	callbox        string
	currentSession *serverPollSession
}

// serverPollSession is a single session of results polling.
type serverPollSession struct {
	client      *manager.CallboxManagerClient
	callbox     string
	savedResult *iperf.Result
	results     chan *manager.IperfServerResult
	ctxCancel   context.CancelFunc
	ready       chan bool
	endTime     time.Time
}

func newServerResultPoller(callbox string, client *manager.CallboxManagerClient) *serverResultPoller {
	return &serverResultPoller{
		client:  client,
		callbox: callbox,
	}
}

// start begins a new polling session in the background.
func (s *serverResultPoller) start(ctx context.Context, cfg *iperf.Config) error {
	if err := s.stop(ctx); err != nil {
		return errors.Wrap(err, "failed to stop existing Iperf server polling session")
	}

	pollCtx, cancel := context.WithTimeout(ctx, cfg.TestTime+testTimeMargin)
	s.currentSession = &serverPollSession{
		client:    s.client,
		callbox:   s.callbox,
		ctxCancel: cancel,
		// callbox returns a max of nSamples + 1 results.
		results: make(chan *manager.IperfServerResult, int(cfg.TestTime/time.Second)+1),
		ready:   make(chan bool),
		endTime: time.Now().Add(cfg.TestTime + testTimeMargin),
	}

	go s.currentSession.poll(pollCtx)
}

// stop stops the currently active polling session (if any).
func (s *serverResultPoller) stop(ctx context.Context) error {
	if s.currentSession != nil {
		return s.currentSession.stop(ctx)
	}

	return nil
}

// fetchResult stops the current polling session and fetches the result.
func (s *serverResultPoller) fetchResult(ctx context.Context) (*iperf.Result, error) {
	if s.currentSession == nil {
		return nil, errors.New("failed to get results from poller, no session found")
	}

	return s.currentSession.fetchResult(ctx)
}

// poll polls the callbox results until either the specified end time or the context has been canceled.
func (s *serverPollSession) poll(ctx context.Context) {
	// close channel when done, a new channel and cancel is created for each session
	defer func() {
		close(s.results)
		s.ready <- true
	}()

	// callbox results are updated every 1 second, poll every 0.5 seconds and ignore duplicate results
	seen := make(map[int]bool)
	for time.Now().Before(s.endTime) {
		result, err := s.client.FetchIperfResult(ctx, &manager.FetchIperfResultRequestBody{Callbox: s.callbox})
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				testing.ContextLog(ctx, "Failed to fetch results from callbox, err: ", err)
			}
			return
		}

		serverResult := result.Servers[0]
		if serverResult != nil && !seen[serverResult.ID] {
			// callbox can only return n results, if we have more. Then something went wrong.
			if len(seen) == cap(s.results) {
				testing.ContextLog(ctx, "WARNING: found more results than expected when polling callbox")
				return
			}

			seen[serverResult.ID] = true
			s.results <- serverResult
		}

		if err := testing.Sleep(ctx, time.Second/2); err != nil {
			if !errors.Is(err, context.Canceled) {
				testing.ContextLog(ctx, "Failed to sleep while polling for results, err: ", err)
			}
			return
		}
	}
}

// stop stops the current background polling session (if one exists).
func (s *serverPollSession) stop(ctx context.Context) error {
	// cancel any polling operation on the server
	if s.ctxCancel != nil {
		s.ctxCancel()
		s.ctxCancel = nil

		for {
			select {
			case <-s.ready:
				return nil
			default:
				if err := testing.Sleep(ctx, time.Second); err != nil {
					return errors.Wrap(err, "failed to wait for poller to stop")
				}
			}
		}
	}

	return nil
}

// fetchResult fetches the results from the resultChannel
func (s *serverPollSession) fetchResult(ctx context.Context) (*iperf.Result, error) {
	// multiple calls to fetchResult will clear out the channel
	if s.savedResult != nil {
		return s.savedResult, nil
	}

	// make sure any polling routines are stopped before attempting to read the channel
	if err := s.stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop existing Iperf server polling session")
	}

	sampleCount := len(s.results)
	if sampleCount == 0 {
		return nil, errors.New("failed to get results from callbox, no useable data found")
	}

	var totalThroughput float64
	for result := range s.results {
		totalThroughput += result.Throughput
	}

	s.savedResult = &iperf.Result{
		Duration:   time.Duration(sampleCount) * time.Second,
		Throughput: iperf.BitRate(totalThroughput / float64(sampleCount)),
	}

	return s.savedResult, nil
}

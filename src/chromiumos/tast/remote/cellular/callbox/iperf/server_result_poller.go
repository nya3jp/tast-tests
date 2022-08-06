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
	currentSession *serverPollingSession
}

// serverPollSession is a single session of results polling.
type serverPollingSession struct {
	client      *manager.CallboxManagerClient
	callbox     string
	savedResult *iperf.Result
	savedError  error
	resultsCh   chan *manager.IperfServerResult
	ctxCancel   context.CancelFunc
	readyCh     chan bool
	errCh       chan error
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

	pollCtx, cancel := context.WithTimeout(ctx, cfg.TestTime+commandTimeoutMargin+testTimeMargin)
	s.currentSession = &serverPollingSession{
		client:    s.client,
		callbox:   s.callbox,
		ctxCancel: cancel,
		// callbox returns a max of TestTime/time.Second samples
		resultsCh: make(chan *manager.IperfServerResult, int(cfg.TestTime/time.Second)),
		readyCh:   make(chan bool, 1),
		errCh:     make(chan error, 1),
		endTime:   time.Now().Add(cfg.TestTime + testTimeMargin),
	}

	go s.currentSession.poll(pollCtx)
	return nil
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
func (s *serverPollingSession) poll(ctx context.Context) {
	// close channel when done, a new channel and cancel is created for each session
	defer func() {
		close(s.resultsCh)
		close(s.errCh)
		s.readyCh <- true
	}()

	// callbox results are updated every 1 second, poll every 0.5 seconds and ignore duplicate results
	seen := make(map[int]bool)
	for time.Now().Before(s.endTime) {
		result, err := s.client.FetchIperfResult(ctx, &manager.FetchIperfResultRequestBody{Callbox: s.callbox})
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				s.errCh <- errors.Wrap(err, "failed to fetch results from callbox")
			}
			return
		}

		if result == nil || result.Servers == nil || len(result.Servers) < 1 {
			s.errCh <- errors.Wrap(err, "failed to fetch server results from callbox, result was nil")
			return
		}

		serverResult := result.Servers[0]
		if serverResult != nil && !seen[serverResult.ID] {
			// callbox can only return at max TestTime server results
			if len(seen) == cap(s.resultsCh) {
				s.errCh <- errors.New("failed to poll server results from callbox, more results than expected")
				return
			}

			seen[serverResult.ID] = true
			s.resultsCh <- serverResult
		}

		if err := testing.Sleep(ctx, time.Second/2); err != nil {
			if !errors.Is(err, context.Canceled) {
				s.errCh <- errors.Wrap(err, "failed to sleep while polling for results")
			}
			return
		}
	}
}

// stop stops the current background polling session (if one exists).
func (s *serverPollingSession) stop(ctx context.Context) error {
	// cancel any polling operation on the server
	if s.ctxCancel != nil {
		s.ctxCancel()
		s.ctxCancel = nil

		if err := testing.Poll(ctx, func(ctx context.Context) error {
			select {
			case <-s.readyCh:
				return nil
			default:
				return errors.New("still waiting for poller to exit")
			}
		}, &testing.PollOptions{Interval: 1 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to stop callbox results poller")
		}
	}

	return nil
}

// fetchResult fetches the results from the resultChannel
func (s *serverPollingSession) fetchResult(ctx context.Context) (*iperf.Result, error) {
	// multiple calls to fetchResult will clear out result & error channels
	if s.savedResult != nil {
		return s.savedResult, nil
	}

	if s.savedError != nil {
		return nil, s.savedError
	}

	// wait until end time to give poller a chance to stop naturally and avoid killing a fetch prematurely
	if err := testing.Sleep(ctx, s.endTime.Sub(time.Now())); err != nil {
		return nil, errors.Wrap(err, "failed to sleep while waiting for server to stop")
	}

	// make sure any polling routines are stopped before attempting to read the channel
	if err := s.stop(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to stop existing Iperf server polling session")
	}

	if len(s.errCh) > 0 {
		s.savedError = errors.Wrap(<-s.errCh, "failed while polling callbox results")
		return nil, s.savedError
	}

	if len(s.resultsCh) != cap(s.resultsCh) {
		return nil, errors.Errorf("failed to get results from callbox, missing data: got %d samples, expected %d", len(s.resultsCh), cap(s.resultsCh))
	}

	var totalThroughput float64
	var totalLoss float64
	sampleCount := len(s.resultsCh)
	for result := range s.resultsCh {
		totalThroughput += result.Throughput
		totalLoss += result.PercentLoss
	}

	s.savedResult = &iperf.Result{
		Duration:    time.Duration(sampleCount) * time.Second,
		Throughput:  iperf.BitRate(totalThroughput / float64(sampleCount)),
		PercentLoss: totalLoss / float64(sampleCount),
	}

	return s.savedResult, nil
}

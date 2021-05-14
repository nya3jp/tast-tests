// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"

	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// ContinuousSession is a Session optimized for continuouns running of single config,
// minimizing delays between Run()s.
type ContinuousSession struct {
	session   *Session
	runner    *runner
	cfg       Config
	udpMaerts bool
}

// NewContinuousSession creates a new continuous netperf session.
// The difference from normal session is that it uses a single runner for all
// runs and Run() runs the runner excatly one time.
func NewContinuousSession(ctx context.Context,
	clientConn *ssh.Conn, clientIP string,
	serverConn *ssh.Conn, serverIP string,
	cfg Config) (*ContinuousSession, error) {
	sessionLock.Lock()
	nps := &ContinuousSession{session: &Session{
		client: RunnerHost{conn: clientConn, ip: clientIP},
		server: RunnerHost{conn: serverConn, ip: serverIP},
	},
		cfg: cfg,
	}
	// For some reason, netperf does not support UDP_MAERTS test.
	if cfg.TestType == TestTypeUDPMaerts {
		// But this is just a reversed UDP_STREAM.
		cfg.TestType = TestTypeUDPStream
		cfg.Reverse = !cfg.Reverse
		nps.udpMaerts = true
	}

	// Ignore any results - if no server was running, the command will return error.
	_ = prepareServer(ctx, nps.session.server)
	// This cleans leftovers of UDP_MAERTS test.
	_ = prepareServer(ctx, nps.session.client)

	var err error
	nps.runner, err = newRunner(ctx, nps.session.client, nps.session.server, cfg)
	return nps, err
}

// Run runs netperf runner once.
func (s *ContinuousSession) Run(ctx context.Context) (History, error) {
	testing.ContextLogf(ctx, "Performing %s measurements in netperf session",
		s.cfg.HumanReadableTag())
	history := History{}
	s.session.runs++

	// The goal is to return result quickly. It is the responsibility
	// of the caller to make proper use of the data.
	result, err := s.runner.run(ctx, 1)
	if err != nil {
		testing.ContextLog(ctx, "Failed, err: ", err)
		return nil, err
	}

	// Handle UDP_MAERTS case, restore the original type.
	if s.udpMaerts {
		result.TestType = TestTypeUDPMaerts
	}

	// Accumulate result.
	history = append(history, result)

	return history, nil
}

// WarmupStations is running short netperf bursts to make sure the station
// is "warmed up" - have a stable, maximum throughput.
// Returns error when too many errors are returned from the runner.
// Otherwise returns nil when results are stable enough or `warmupMaxSamples` runs.
func (s *ContinuousSession) WarmupStations(ctx context.Context) error {
	s.session.runs++
	return s.runner.warmup(ctx)
}

// Close closes the session.
func (s *ContinuousSession) Close(ctx context.Context) {
	s.runner.close(ctx)
	s.session.client = RunnerHost{}
	s.session.server = RunnerHost{}
	sessionLock.Unlock()
	testing.ContextLog(ctx, "Netperf session closed")
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	maxDeviationFraction   = 0.03
	measurementMaxSamples  = 10
	measurementMaxFailures = 2
	measurementMinSamples  = 3
	warmupSampleTime       = 2 * time.Second
	warmupWindowSize       = 2
	warmupMaxSamples       = 10
	retryCount             = 3
)

// Session contains session data for running Runner.
type Session struct {
	client         RunnerHost
	server         RunnerHost
	ignoreFailures bool
}

// History is a set of netperf results.
type History []*Result

// NewSession creates a new netperf session.
// In each session, various tests with different configurations can be run.
func NewSession(
	clientConn *ssh.Conn,
	clientIP string,
	serverConn *ssh.Conn,
	serverIP string) *Session {

	nps := &Session{
		client: RunnerHost{conn: clientConn, ip: clientIP},
		server: RunnerHost{conn: serverConn, ip: serverIP},
	}
	return nps
}

// Run netperf runner with a particular test configuration.
func (s *Session) Run(ctx context.Context, cfg Config) (History, error) {
	// For some reason, netperf does not support UDP_MAERTS test.
	udpMaerts := false
	if cfg.TestType == TestTypeUDPMaerts {
		// But this is just a reversed UDP_STREAM.
		cfg.TestType = TestTypeUDPStream
		cfg.Reverse = !cfg.Reverse
		udpMaerts = true
	}

	testing.ContextLogf(ctx, "Performing %s measurements in netperf session",
		cfg.HumanReadableTag)
	history := History{}
	var finalResult *Result

	// Create new runner each time session Run() is called,
	// because config may require runner to swap client and server.
	runner, err := newRunner(ctx, s.client, s.server, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize runner")
	}
	defer runner.Close(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	// The goal is to accumulate enough stable perf results to to be sure
	// we're accurate (all deviations are small enough), but don't use
	// too many (measurementMaxSamples) attempts to achieve that.
	for noneCount := 0; len(history)+noneCount < measurementMaxSamples; {
		result, err := runner.run(ctx, retryCount)
		if err != nil {
			testing.ContextLog(ctx, "Failed, err: ", err)
			noneCount++

			// Might occur when, e.g., signal strength is too low.
			// Give up after measurementMaxFailures failures.
			if noneCount > measurementMaxFailures {
				return nil, errors.Wrapf(err, "too many failures (%d), aborting",
					noneCount)
			}
			continue
		}
		// Handle UDP_MAERTS case, restore the original type.
		if udpMaerts {
			result.testType = TestTypeUDPMaerts
		}

		// Accumulate result.
		history = append(history, result)
		// There's no point in calculating deviations for too small population.
		if len(history) < measurementMinSamples {
			continue
		}

		// Calculate deviations from available history.
		finalResult, err := FromSamples(ctx, history)
		if err != nil {
			testing.ContextLog(ctx, "Failed calculating stats, err: ", err)
		} else if finalResult.AllDeviationsLessThanFraction(maxDeviationFraction) {
			// If deviations are satisfactory, stop accumulationg samples.
			break
		}
	}

	// If, for some reason, we could not have calculated final result but have
	// at least rudimentary history, let's calcualte it with what we have.
	if finalResult == nil && len(history) > 0 {
		finalResult, err = FromSamples(ctx, history)
		if err != nil {
			return nil, errors.Wrap(err, "error calculating statistics from samples")
		}
	}

	testing.ContextLogf(ctx, "Took averaged measurement %s", finalResult.String())
	return history, nil
}

func (s *Session) warmupWifiPart(ctx context.Context, cfg Config) error {
	runner, err := newRunner(ctx, s.client, s.server, cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize runner")
	}
	defer runner.Close(ctx)
	ctx, cancel := ctxutil.Shorten(ctx, time.Second)
	defer cancel()

	var warmupHistory History
	errCount := 0
	for len(warmupHistory) < warmupMaxSamples {
		ret, err := runner.run(ctx, retryCount)
		if err != nil {
			errCount++
			if errCount > measurementMaxFailures {
				return errors.Wrapf(err, "too many failures (%d), aborting",
					errCount)
			}
			continue
		}
		warmupHistory = append(warmupHistory, ret)
		if len(warmupHistory) > 2*warmupWindowSize {
			// Grab 2 * warmupWindowSize samples, divided into the most
			// recent chunk and the chunk before that.
			start := len(warmupHistory) - 2*warmupWindowSize
			middle := len(warmupHistory) - warmupWindowSize
			pastResult, err := FromSamples(ctx,
				warmupHistory[start:middle])
			if err != nil {
				return errors.Wrap(err, "error calculating throughput")
			}
			recentResult, err := FromSamples(ctx, warmupHistory[middle:])
			if recentResult.measurements[CategoryThroughput] <
				(pastResult.measurements[CategoryThroughput] +
					pastResult.measurements[CategoryThroughputDev]) {
				testing.ContextLog(ctx, "Rate controller is warmed")
				return nil
			}
		}
	}
	testing.ContextLog(ctx, "Warning: Did not completely warmup the WiFi part")
	return nil
}

// WarmupStations is running short netperf bursts in both directions to make sure
// both station are "warmed up" - have a stable, maximum throughput.
func (s *Session) WarmupStations(ctx context.Context) error {
	err := s.warmupWifiPart(ctx, Config{
		TestTime: warmupSampleTime,
		TestType: TestTypeTCPStream,
	})
	if err != nil {
		return errors.Wrap(err, "error warming up client")
	}
	err = s.warmupWifiPart(ctx, Config{
		TestTime: warmupSampleTime,
		TestType: TestTypeTCPMaerts,
	})
	if err != nil {
		return errors.Wrap(err, "error warming up server")
	}
	return nil
}

// Close closes the session.
func (s *Session) Close(ctx context.Context) {
	s.client = RunnerHost{}
	s.server = RunnerHost{}
	testing.ContextLog(ctx, "Netperf session closed")
}

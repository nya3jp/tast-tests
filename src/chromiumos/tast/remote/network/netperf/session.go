// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	maxDeviationFraction   = 0.03
	measurementMaxSamples  = 10
	measurementMaxFailures = 2
	measurementMinSamples  = 3
	warmupSampleTime       = "2s"
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
// In each session, variuos tests with different configurations can be run.
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
	if cfg.TestType == TestTypeUDPMaerts {
		// But this is just a reversed UDP_STREAM.
		cfg.TestType = TestTypeUDPStream
		cfg.Reverse = true
	}

	testing.ContextLogf(ctx, "Performing %s measurements in netperf session",
		cfg.HumanReadableTag)
	history := History{}
	// noneCount = 0
	var finalResult *Result
	runner, err := NewRunner(ctx, s.client, s.server, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize runner")
	}

	defer func(ctx context.Context) {
		runner.Close(ctx)
	}(ctx)

	for noneCount := 0; len(history)+noneCount < measurementMaxSamples; {
		result, err := runner.Run(ctx, retryCount)
		if err != nil {
			testing.ContextLog(ctx, "Failed, err: ", err)
			noneCount++

			// Might occur when, e.g., signal strength is too low.
			if noneCount > measurementMaxFailures {
				return nil, errors.Wrapf(err, "too many failures (%d), aborting",
					noneCount)
			}
			continue
		}
		// Handle UDP_MAERTS case.
		if cfg.Reverse && cfg.TestType == TestTypeUDPStream {
			result.testType = TestTypeUDPMaerts
		}

		history = append(history, result)
		if len(history) < measurementMinSamples {
			continue
		}

		finalResult, err := fromSamples(ctx, history)
		if err != nil {
			testing.ContextLog(ctx, "Failed calculating stats, err: ", err)
		}
		if finalResult.AllDeviationsLessThanFraction(
			maxDeviationFraction) {
			break
		}
	}

	if finalResult == nil {
		ret, err := fromSamples(ctx, history)
		if err != nil {
			return nil, errors.Wrap(err, "error calculating statistics from samples")
		}
		finalResult = ret
	}

	testing.ContextLogf(ctx, "Took averaged measurement %s", finalResult.String())
	return history, nil
}

func (s *Session) warmupWifiPart(ctx context.Context, cfg Config) error {
	runner, err := NewRunner(ctx, s.client, s.server, cfg)
	if err != nil {
		return errors.Wrap(err, "failed to initialize runner")
	}
	defer func(ctx context.Context) {
		runner.Close(ctx)
	}(ctx)
	var warmupHistory History
	errCount := 0
	for len(warmupHistory) < warmupMaxSamples {
		ret, err := runner.Run(ctx, retryCount)
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
			pastResult, err := fromSamples(ctx,
				warmupHistory[start:middle])
			if err != nil {
				return errors.Wrap(err, "error calculating throughput")
			}
			recentResult, err := fromSamples(ctx, warmupHistory[middle:])
			if recentResult.measurements["throughput"] <
				(pastResult.measurements["throughput"] +
					pastResult.measurements["throughput dev"]) {
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
	dur, _ := time.ParseDuration(warmupSampleTime)
	err := s.warmupWifiPart(ctx, Config{
		TestTime: dur,
		TestType: TestTypeTCPStream,
	})
	if err != nil {
		return errors.Wrap(err, "error warming up client")
	}
	err = s.warmupWifiPart(ctx, Config{
		TestTime: dur,
		TestType: TestTypeTCPMaerts,
	})
	if err != nil {
		return errors.Wrap(err, "error warming up server")
	}
	return nil
}

// Close closes the session.
func (s *Session) Close(ctx context.Context) {
	testing.ContextLog(ctx, "Netperf session closed")
}

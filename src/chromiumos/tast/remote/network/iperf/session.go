// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	maxMeasurementSamples  = 10
	maxMeasurementFailures = 2
	minMeasurementSamples  = 3
	maxStandardDeviation   = 0.03
)

// Session represents an IPerf session consisting of multiple runs.
type Session struct {
	client Client
	server Server
}

// History is a set of netperf results.
type History []*Result

// NewSession creates a new Iperf session.
func NewSession(client Client, server Server) *Session {
	return &Session{
		client: client,
		server: server,
	}
}

// Run runs multiple Iperf tests and takes the average performance values.
func (s *Session) Run(ctx context.Context, config *Config) (History, error) {
	testing.ContextLogf(ctx, "Performing %s measurements in Iperf session", config.TestType)

	history := History{}
	failureCount := 0
	var finalResult *Result
	runner := NewRunner(config, s.client, s.server)
	for len(history)+failureCount < maxMeasurementSamples &&
		failureCount < maxMeasurementFailures &&
		(finalResult == nil || finalResult.StdDeviation > maxStandardDeviation) {
		result, err := runner.Run(ctx, 3)
		if err != nil {
			failureCount++
		} else {
			testing.ContextLogf(ctx, "Completed Iperf measurement, throughput: %f loss: %v", result.Throughput, result.PercentLoss)
			history = append(history, result)
			if len(history) >= minMeasurementSamples {
				finalResult, err = NewResultFromHistory(history)
				if err != nil {
					return nil, errors.Wrap(err, "error calculating statistics from samples")
				}
			}
		}
	}

	if finalResult == nil {
		return nil, errors.New("failed to to measure performance, Iperf failed too many times")
	}

	testing.ContextLogf(ctx, "Took averaged measurement %f +/- %f",
		finalResult.Throughput, finalResult.StdDeviation)

	return history, nil
}

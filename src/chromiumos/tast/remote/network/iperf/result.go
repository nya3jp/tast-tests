// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package iperf

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

const (
	logIDIndex          = 5
	intervalIndex       = 6
	dataTransferedIndex = 7
	percentLossIndex    = 12

	fieldCount    = 8
	fieldCountUDP = 14
)

// Result represents an aggregated set of Iperf results.
type Result struct {
	Duration     time.Duration
	Throughput   BitRate
	PercentLoss  float64
	StdDeviation BitRate
}

func newResultFromOutput(ctx context.Context, output string, config *Config) (*Result, error) {
	var totalByteCount float64
	var totalDuration float64
	var totalLoss float64

	var allErrors error
	count := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, ",")

		// only use client side results for UDP
		if len(fields) < fieldCount || (config.Protocol == ProtocolUDP && len(fields) < fieldCountUDP) {
			continue
		}

		// ignore summary lines
		if logID, err := strconv.Atoi(fields[logIDIndex]); err != nil || logID == -1 {
			continue
		}

		byteCount, err := strconv.ParseFloat(fields[dataTransferedIndex], 64)
		if err != nil {
			allErrors = errors.Wrapf(allErrors, "failed to parse bytes from %q: %v ", fields[dataTransferedIndex], err) // NOLINT
			continue
		}

		duration, err := parseInterval(fields[intervalIndex])
		if err != nil {
			allErrors = errors.Wrapf(allErrors, "failed to parse duration from: %q: %v ", fields[intervalIndex], err) // NOLINT
			continue
		}

		var loss float64
		if config.Protocol == ProtocolUDP {
			loss, err = strconv.ParseFloat(fields[percentLossIndex], 64)
			if err != nil {
				allErrors = errors.Wrapf(allErrors, "failed to parse loss from %q: %v ", fields[percentLossIndex], err) // NOLINT
				continue
			}
		}

		totalDuration += duration
		totalByteCount += byteCount
		totalLoss += loss

		count++
	}

	expectedCount := config.PortCount
	if config.Bidirectional && config.Protocol != ProtocolUDP {
		expectedCount *= 2
	}

	if count != expectedCount {
		return nil, errors.Wrapf(allErrors, "missing data: got %v lines, want %v", count, expectedCount)
	}

	totalDuration = totalDuration / float64(config.PortCount)
	return &Result{
		Duration:    time.Duration(totalDuration / float64(count)),
		PercentLoss: totalLoss / float64(count),
		Throughput:  8 * BitRate(totalByteCount/totalDuration),
	}, allErrors
}

// parseInterval returns the duration from an Iperf interval or -1 if it was unable to parse.
func parseInterval(interval string) (float64, error) {
	bounds := strings.Split(interval, "-")
	if len(bounds) != 2 {
		return 0, errors.Errorf("unable to split duration interval: %v", interval)
	}

	start, err := strconv.ParseFloat(bounds[0], 64)
	if err != nil {
		return 0, errors.Errorf("unable to parse duration start: %v", bounds[0])
	}

	end, err := strconv.ParseFloat(bounds[1], 64)
	if err != nil {
		return 0, errors.Errorf("unable to parse duration end: %v", bounds[1])
	}

	return end - start, nil
}

// NewResultFromHistory returns the average from a set of results.
func NewResultFromHistory(samples []*Result) (*Result, error) {
	count := len(samples)
	if count == 0 {
		return nil, errors.New("received empty samples slice")
	}

	var totalDuration time.Duration
	var meanThroughput float64
	var meanLoss float64
	var stdDev float64

	for _, sample := range samples {
		totalDuration += sample.Duration
		meanThroughput += float64(sample.Throughput) / float64(count)
		meanLoss += sample.PercentLoss / float64(count)
	}

	for _, sample := range samples {
		stdDev += math.Pow(float64(sample.Throughput)-meanThroughput, 2)
	}

	stdDev = math.Sqrt(stdDev / float64(count))

	return &Result{
		Duration:     totalDuration,
		Throughput:   BitRate(meanThroughput),
		PercentLoss:  meanLoss,
		StdDeviation: BitRate(stdDev),
	}, nil
}

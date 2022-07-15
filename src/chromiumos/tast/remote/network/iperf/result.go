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
	Throughput   float64
	PercentLoss  float64
	StdDeviation float64
}

func newResultFromOutput(ctx context.Context, output string, config *Config) (*Result, error) {
	var totalByteCount float64
	var totalDuration float64
	var totalLoss float64

	count := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, ",")

		// only use client side results for UDP
		if len(fields) < fieldCount || (config.isUDP() && len(fields) < fieldCountUDP) {
			continue
		}

		if logID, err := strconv.Atoi(fields[logIDIndex]); err != nil || logID == -1 {
			continue
		}

		byteCount, err := strconv.ParseFloat(fields[dataTransferedIndex], 64)
		if err != nil {
			continue
		}

		duration := parseInterval(fields[intervalIndex])
		if duration < 0 {
			continue
		}

		var loss float64
		if config.isUDP() {
			loss, err = strconv.ParseFloat(fields[percentLossIndex], 64)
			if err != nil {
				continue
			}
		}

		totalDuration += duration
		totalByteCount += byteCount
		totalLoss += loss

		count++
	}

	expectedCount := config.PortCount
	if config.isBidirectional() && !config.isUDP() {
		expectedCount *= 2
	}

	if count != expectedCount {
		return nil, errors.New("missing data")
	}

	totalDuration = totalDuration / float64(config.PortCount)
	return &Result{
		Duration:    time.Duration(totalDuration / float64(count)),
		PercentLoss: totalLoss / float64(count),
		Throughput:  8 * (totalByteCount / totalDuration) / 1000000,
	}, nil
}

// parseInterval returns the duration from an Iperf interval or -1 if it was unable to parse.
func parseInterval(interval string) float64 {
	bounds := strings.Split(interval, "-")
	if len(bounds) != 2 {
		return -1
	}

	start, err := strconv.ParseFloat(bounds[0], 64)
	if err != nil {
		return -1
	}

	end, err := strconv.ParseFloat(bounds[1], 64)
	if err != nil {
		return -1
	}

	return end - start
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
		meanThroughput += sample.Throughput / float64(count)
		meanLoss += sample.Throughput / float64(count)
	}

	for _, sample := range samples {
		stdDev += math.Pow(sample.Throughput-float64(meanThroughput), 2)
	}

	return &Result{
		Duration:     totalDuration,
		Throughput:   meanThroughput,
		PercentLoss:  meanLoss,
		StdDeviation: math.Sqrt(stdDev / float64(count)),
	}, nil
}

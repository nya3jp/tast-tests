// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package netperf

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
)

// Category is a measurement category, used as the key in measurements map.
// Its value is also used as a human-readable tag.
type Category string

const (
	// CategoryThroughput measures throughput in Mbps.
	CategoryThroughput Category = "throughput"
	// CategoryTransactionRate measures transaction rate per s.
	CategoryTransactionRate = "transaction rate"
	// CategoryErrors measures transmission errors.
	CategoryErrors = "errors"
	// CategoryThroughputDev st. deviation of throughput.
	CategoryThroughputDev = "throughput dev"
	// CategoryTransactionRateDev st. deviation of transaction rate.
	CategoryTransactionRateDev = "transaction rate dev"
	// CategoryErrorsDev st. deviation of errors.
	CategoryErrorsDev = "errors dev"
)

// Result is used to carry either single result or its derivative
// (mean/st.dev).
type Result struct {
	TestType TestType
	Duration time.Duration
	// Measurements: throughput, transactionRate, errors or their st.deviations.
	Measurements map[Category]float64
}

// NewResult returns initialized Result.
func NewResult(testType TestType, duration time.Duration) *Result {
	return &Result{
		TestType:     testType,
		Duration:     duration,
		Measurements: make(map[Category]float64),
	}
}

// String returns string representation of the result.
func (r *Result) String() string {
	var measurements []string

	for key, val := range r.Measurements {
		measurements = append(measurements, fmt.Sprintf("%s: %.3f", string(key), val))
	}
	return fmt.Sprintf("{type: %s, duration: %v, measurements:[%s]}",
		r.TestType, r.Duration, strings.Join(measurements, ", "))
}

// parseNetperfOutput creates result struct directly from netperf output string.
func parseNetperfOutput(ctx context.Context, testType TestType, results string,
	duration time.Duration) (*Result, error) {
	lines := strings.Split(results, "\n")
	dataRegEx := regexp.MustCompile("^[0-9]+")
	var dataLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if dataRegEx.MatchString(line) {
			dataLines = append(dataLines, line)
		}
	}

	ret := NewResult(testType, duration)
	switch testType {
	// Parses the following (works for both TCP_STREAM, TCP_MAERTS and
	// TCP_SENDFILE) and sets up throughput.

	// TCP STREAM TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET to \
	// foo.bar.com (10.10.10.3) port 0 AF_INET
	// Recv   Send    Send
	// Socket Socket  Message  Elapsed
	// Size   Size    Size     Time     Throughput
	// bytes  bytes   bytes    secs.    10^6bits/sec
	//
	// 87380  16384  16384    2.00      941.28
	case TestTypeTCPMaerts, TestTypeTCPSendfile, TestTypeTCPStream:
		if len(dataLines) < 1 || len(strings.Fields(dataLines[0])) < 5 {
			return nil, errors.New("no valid results found in the output")
		}
		ret.Measurements[CategoryThroughput], _ =
			strconv.ParseFloat(strings.Fields(dataLines[0])[4], 64)

	// Parses the following and returns a tuple containing throughput
	// and the number of errors.

	// UDP UNIDIRECTIONAL SEND TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET \
	// to foo.bar.com (10.10.10.3) port 0 AF_INET
	// Socket  Message  Elapsed      Messages
	// Size    Size     Time         Okay Errors   Throughput
	// bytes   bytes    secs            #      #   10^6bits/sec
	//
	// 129024   65507   2.00         3673      0     961.87
	// 131072           2.00         3673            961.87
	case TestTypeUDPStream, TestTypeUDPMaerts:
		if len(dataLines) < 2 || len(strings.Fields(dataLines[0])) < 5 ||
			len(strings.Fields(dataLines[1])) < 3 {
			return nil, errors.New("no valid results found in the output")
		}
		// We take throuput from the receiving side (2nd line).
		ret.Measurements[CategoryThroughput], _ = strconv.ParseFloat(strings.Fields(dataLines[1])[3], 64)
		// Errors are: transmission errors and all messages that were sent but not received.
		txErrors, _ := strconv.ParseFloat(strings.Fields(dataLines[0])[4], 64)
		txMsgs, _ := strconv.ParseFloat(strings.Fields(dataLines[0])[3], 64)
		rxMsgs, _ := strconv.ParseFloat(strings.Fields(dataLines[1])[2], 64)
		ret.Measurements[CategoryErrors] = txErrors + txMsgs - rxMsgs

	// Parses the following which works for both rr (TCP and UDP)
	// and crr tests and returns a singleton containing transfer rate.

	// TCP REQUEST/RESPONSE TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET \
	// to foo.bar.com (10.10.10.3) port 0 AF_INET
	// Local /Remote
	// Socket Size   Request  Resp.   Elapsed  Trans.
	// Send   Recv   Size     Size    Time     Rate
	// bytes  Bytes  bytes    bytes   secs.    per sec
	//
	// 16384  87380  1        1       2.00     14118.53
	// 16384  87380
	case TestTypeTCPCRR, TestTypeTCPRR, TestTypeUDPRR:
		if len(dataLines) < 1 || len(strings.Fields(dataLines[0])) < 6 {
			return nil, errors.New("no valid results found in the output")
		}
		ret.Measurements[CategoryTransactionRate], _ =
			strconv.ParseFloat(strings.Fields(dataLines[0])[5], 64)
	}
	return ret, nil
}

// hasCategory check if samples set contains measurements of certain category,
func hasCategory(samples []*Result, category Category) bool {
	if len(samples) == 0 {
		return false
	}
	for _, sample := range samples {
		if _, ok := sample.Measurements[category]; ok {
			return true
		}
	}
	return false
}

// calculateStats calculates mean/st.dev. for any given category in samples.
func calculateStats(samples []*Result, category Category) (mean, deviation float64, validSamples int) {
	var sum float64
	numSamples := 0
	for _, sample := range samples {
		sum += sample.Measurements[category]
		numSamples++
	}
	mean = sum / float64(numSamples)
	var dev float64
	if numSamples > 1 {
		for _, sample := range samples {
			dev += math.Pow(sample.Measurements[category]-mean, 2)
		}
		dev = math.Sqrt(dev / float64(numSamples))
	}
	return mean, dev, numSamples
}

// AggregateSamples creates aggregate result out of slice of results
func AggregateSamples(ctx context.Context, samples []*Result) (*Result, error) {
	samplesLen := len(samples)
	if samplesLen < 1 {
		return nil, errors.New("received empty samples slice")
	}
	duration := samples[0].Duration
	numSamples := 0
	validSamples := 0

	ret := NewResult(samples[0].TestType, 0)

	// Check correctness of input.
	for _, sample := range samples {
		if sample.TestType != ret.TestType {
			return nil,
				errors.New("boldly refusing to compare samples from different test types")
		}
		if sample.Duration != duration {
			return nil,
				errors.New("comparing samples with different durations not supported")
		}
	}

	if hasCategory(samples, CategoryThroughput) {
		ret.Measurements[CategoryThroughput],
			ret.Measurements[CategoryThroughputDev],
			validSamples = calculateStats(samples, CategoryThroughput)
		numSamples += validSamples
	}
	if hasCategory(samples, CategoryTransactionRate) {
		ret.Measurements[CategoryTransactionRate],
			ret.Measurements[CategoryTransactionRateDev],
			validSamples = calculateStats(samples, CategoryTransactionRate)
		// The result of the same type consists of either throughput or transaction rate,
		// we may easily use sum to get total number of valid samples.
		numSamples += validSamples
	}
	if hasCategory(samples, CategoryErrors) {
		// Errors always come together with transaction rate,
		// no need to count valid samples.
		ret.Measurements[CategoryErrors], ret.Measurements[CategoryErrorsDev], _ =
			calculateStats(samples, CategoryErrors)
	}
	ret.Duration = time.Duration(numSamples) * duration

	return ret, nil
}

// withinCoefficientVariation checks if important deviations in the result
// are less than a given fraction of the mean.
func (r *Result) withinCoefficientVariation(cv float64) bool {
	for _, m := range []struct {
		val Category
		dev Category
	}{
		{CategoryThroughput, CategoryThroughputDev},
		{CategoryTransactionRate, CategoryTransactionRateDev},
		// Errors are left out on purpose, their granularity is not enough
		// to provide a valid cv.
	} {
		value, ok := r.Measurements[m.val]
		if !ok {
			continue
		}
		dev, ok := r.Measurements[m.dev]
		if !ok {
			continue
		}
		if value == 0 && dev == 0 {
			// 0/0 is undefined, but take this to be good for our purposes.
			continue
		}
		if dev != 0 && value == 0 {
			// Deviation is non-zero, but the average is 0. Deviation
			// as a cv of the value is undefined but in theory
			// a "very large number."
			return false
		}
		if dev/value >= cv {
			return false
		}
	}
	return true
}

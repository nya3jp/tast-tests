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

// Result is used to carry either single result or its derivative
// (mean/st.dev).
type Result struct {
	testType TestType
	duration time.Duration
	// measurements: throughput, transactionRate, errors or their st.deviations.
	measurements map[string]float64
}

// NewResult returns initialized Result.
func NewResult(testType TestType, duration time.Duration) *Result {
	return &Result{
		testType:     testType,
		duration:     duration,
		measurements: make(map[string]float64),
	}
}

// String returns string representation of the result.
func (r *Result) String() string {
	ret := fmt.Sprintf("{type: %s, duration: %v, measurements:[",
		r.testType, r.duration)

	var measurements []string

	for key, val := range r.measurements {
		measurements = append(measurements, fmt.Sprintf("%s: %.3f", key, val))
	}
	return ret + strings.Join(measurements, ", ") + "]"
}

// FromResults creates result struct directly from netperf output string.
func FromResults(ctx context.Context, testType TestType, results string, duration time.Duration) *Result {
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

	// 87380  16384  16384    2.00      941.28
	case TestTypeTCPMaerts:
		fallthrough
	case TestTypeTCPSendfile:
		fallthrough
	case TestTypeTCPStream:
		ret.measurements["throughput"], _ =
			strconv.ParseFloat(strings.Fields(dataLines[0])[4], 64)
		// break

	// Parses the following and returns a tuple containing throughput
	// and the number of errors.

	// UDP UNIDIRECTIONAL SEND TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET \
	// to foo.bar.com (10.10.10.3) port 0 AF_INET
	// Socket  Message  Elapsed      Messages
	// Size    Size     Time         Okay Errors   Throughput
	// bytes   bytes    secs            #      #   10^6bits/sec

	// 129024   65507   2.00         3673      0     961.87
	// 131072           2.00         3673            961.87
	case TestTypeUDPStream:
		fallthrough
	case TestTypeUDPMaerts:
		tokens := strings.Fields(dataLines[0])
		ret.measurements["throughput"], _ = strconv.ParseFloat(tokens[5], 64)
		ret.measurements["errors"], _ = strconv.ParseFloat(tokens[4], 64)
		//break

	// Parses the following which works for both rr (TCP and UDP)
	// and crr tests and returns a singleton containing transfer rate.

	// TCP REQUEST/RESPONSE TEST from 0.0.0.0 (0.0.0.0) port 0 AF_INET \
	// to foo.bar.com (10.10.10.3) port 0 AF_INET
	// Local /Remote
	// Socket Size   Request  Resp.   Elapsed  Trans.
	// Send   Recv   Size     Size    Time     Rate
	// bytes  Bytes  bytes    bytes   secs.    per sec

	// 16384  87380  1        1       2.00     14118.53
	// 16384  87380
	case TestTypeTCPCRR:
		fallthrough
	case TestTypeTCPRR:
		fallthrough
	case TestTypeUDPRR:
		ret.measurements["transaction rate"], _ =
			strconv.ParseFloat(strings.Fields(dataLines[0])[5], 64)
	}
	return ret
}

// getStats calculates mean/st.dev. for any given category in samples.
func getStats(samples []*Result, category string) (mean, deviation float64) {
	var sum float64
	for _, sample := range samples {
		sum += sample.measurements[category]
	}
	numSamples := float64(len(samples))
	mean = sum / numSamples
	var dev float64
	if numSamples > 1 {
		for _, sample := range samples {
			dev += math.Pow(sample.measurements[category]-mean, 2)
		}
		dev = math.Sqrt(dev / numSamples)
	}
	return mean, dev
}

// fromSamples creates aggregate result out of slice of results
func fromSamples(ctx context.Context, samples []*Result) (*Result, error) {
	samplesLen := len(samples)
	if samplesLen < 1 {
		return nil, errors.New("received empty samples slice")
	}
	duration := samples[0].duration

	ret := NewResult(samples[0].testType, time.Duration(samplesLen)*duration)

	// A bit of sanity check.
	for _, sample := range samples {
		if sample.testType != ret.testType {
			return nil,
				errors.New("boldly refusing to compare samples from different test types")
		}
		if sample.duration != duration {
			return nil,
				errors.New("comparing samples with different durations not supported")
		}
	}

	ret.measurements["throughput"], ret.measurements["throughput dev"] =
		getStats(samples, "throughput")
	ret.measurements["transaction rate"], ret.measurements["transaction rate dev"] =
		getStats(samples, "transaction rate")
	ret.measurements["errors"], ret.measurements["errors dev"] =
		getStats(samples, "errors")

	return ret, nil
}

// AllDeviationsLessThanFraction checks if all possible deviations in the result
// are less than a given value.
func (r *Result) AllDeviationsLessThanFraction(fraction float64) bool {
	measurements := []string{"throughput", "errors", "transaction rate"}
	for _, measurement := range measurements {
		value, ok := r.measurements[measurement]
		if !ok {
			continue
		}
		dev, ok := r.measurements[measurement+"Dev"]
		if !ok {
			continue
		}
		if value == 0 && dev == 0 {
			// 0/0 is undefined, but take this to be good for our purposes.
			continue
		}
		if dev != 0 && value == 0 {
			// Deviation is non-zero, but the average is 0. Deviation
			// as a fraction of the value is undefined but in theory
			// a "very large number."
			return false
		}
		if dev/value > fraction {
			return false
		}
	}
	return true
}

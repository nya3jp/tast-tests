// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"io/ioutil"
	"math"
	"regexp"
	"strconv"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
)

// LoopbackLatencyResult is a struct to hold the latencies result from
// multiple runs of loopback_latency tests.
type LoopbackLatencyResult struct {
	ExpectedLoops     int
	MeasuredLatencies []float64
	ReportedLatencies []float64
}

// GetNumOfValidLoops return number of successful latency result.
func (r LoopbackLatencyResult) GetNumOfValidLoops() int {
	if len(r.MeasuredLatencies) < len(r.ReportedLatencies) {
		return len(r.MeasuredLatencies)
	}
	return len(r.ReportedLatencies)
}

// Sample output:
//
//	Assign cap_dev hw:0,0
//	Assign play_dev hw:0,0
//	Found audio
//	Played at 1631853475 67453, 8192 delay
//	Capture at 1631853475 408149, 4096 delay sample 3904
//	Measured Latency: 340696 uS
//	Reported Latency: 174666 uS
var measuredRe = regexp.MustCompile("Measured Latency: [0-9]+ uS")
var reportedRe = regexp.MustCompile("Reported Latency: [0-9]+ uS")

func extractNumbers(strs []string) ([]float64, error) {
	extractRe := regexp.MustCompile("[0-9]+")
	var nums []float64
	for _, numberStr := range strs {
		num, err := strconv.Atoi(extractRe.FindString(numberStr))
		if err != nil {
			return []float64{}, errors.Wrap(err, "atoi failed")
		}
		nums = append(nums, float64(num))
	}
	return nums, nil
}

// ParseLoopbackLatencyResult parse the output from loopback_latency and return
// LoopbackLatencyResult struct on success.
func ParseLoopbackLatencyResult(filepath string, loop int) (LoopbackLatencyResult, error) {
	var err error
	result := LoopbackLatencyResult{
		ExpectedLoops: loop,
	}

	loopbackLogBytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return result, errors.Errorf("failed to read loopback log: %s", err)
	}
	loopbackLog := string(loopbackLogBytes)

	result.MeasuredLatencies, err = extractNumbers(measuredRe.FindAllString(loopbackLog, -1))
	if err != nil {
		return result, errors.Errorf("Extract measured latency failed: %s", err)
	}

	result.ReportedLatencies, err = extractNumbers(reportedRe.FindAllString(loopbackLog, -1))
	if err != nil {
		return result, errors.Errorf("Extract reported latency failed: %s", err)
	}

	return result, nil
}

func minAndMax(nums []float64) (float64, float64) {
	min := math.Inf(1)
	max := math.Inf(-1)
	for _, n := range nums {
		max = math.Max(max, n)
		min = math.Min(min, n)
	}
	return min, max
}

// UpdatePerfValuesFromResult store the latency result in the perfValues to be saved
// to crosbolt.
func UpdatePerfValuesFromResult(perfValues *perf.Values, result LoopbackLatencyResult, bufferSize string) {
	perfValues.Set(
		perf.Metric{
			Name:      "nfail_" + bufferSize,
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
		}, float64(result.ExpectedLoops-result.GetNumOfValidLoops()))

	if result.ExpectedLoops != result.GetNumOfValidLoops() {
		return
	}

	measuredMin, measuredMax := minAndMax(result.MeasuredLatencies)
	reportedMin, reportedMax := minAndMax(result.ReportedLatencies)

	perfValues.Set(
		perf.Metric{
			Name:      "measured_latency_" + bufferSize,
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, result.MeasuredLatencies...)
	perfValues.Set(
		perf.Metric{
			Name:      "reported_latency_" + bufferSize,
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
			Multiple:  true,
		}, result.ReportedLatencies...)
	// crosbolt, please calculate the mins and maxes for multiple values :|
	// go/crosbolt-result-parser-g3doc#results-chartjson
	// min and max are important as latency can spike.
	perfValues.Set(
		perf.Metric{
			Name:      "measured_latency_" + bufferSize + "_min",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
		}, measuredMin)
	perfValues.Set(
		perf.Metric{
			Name:      "measured_latency_" + bufferSize + "_max",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
		}, measuredMax)
	perfValues.Set(
		perf.Metric{
			Name:      "reported_latency_" + bufferSize + "_min",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
		}, reportedMin)
	perfValues.Set(
		perf.Metric{
			Name:      "reported_latency_" + bufferSize + "_max",
			Unit:      "uS",
			Direction: perf.SmallerIsBetter,
		}, reportedMax)
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package encode

import (
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

const (
	// The log name suffix of frame statistics.
	frameStatsSuffix = "frame-data.csv"
	// The log name suffix of dumpping log from test binary.
	testLogSuffix = "test.log"

	// Maximum time to wait for CPU to become idle.
	waitIdleCPUTimeout = 30 * time.Second
	// Average usage below which CPU is considered idle.
	idleCPUUsagePercent = 10.0
	// Time to wait for CPU to stabilize after launching test binary.
	stabilizationDuration = 1 * time.Second
	// Duration of the interval during which CPU usage will be measured.
	measurementDuration = 1 * time.Second
	// The log name of recording CPU usage.
	cpuLog = "cpu.log"

	// Performance keys:
	keyFPS             = "fps"
	keyEncodeLatency50 = "encode_latency.50_percentile"
	keyEncodeLatency75 = "encode_latency.75_percentile"
	keyEncodeLatency95 = "encode_latency.95_percentile"
	keyCPUUsage        = "cpu_usage"

	// Units of performance values:
	unitMicroSecond = "us"
	unitPercent     = "precent"
	unitFPS         = "fps"
	unitSSIM        = "ssim"
	unitPSNR        = "psnr"
)

var regExpFPS = regexp.MustCompile(`(?m)^Measured encoder FPS: ([+\-]?[0-9.]+)$`)
var regExpEncodeLatency50 = regexp.MustCompile(`(?m)^Encode latency for the 50th percentile: (\d+) us$`)
var regExpEncodeLatency75 = regexp.MustCompile(`(?m)^Encode latency for the 75th percentile: (\d+) us$`)
var regExpEncodeLatency95 = regexp.MustCompile(`(?m)^Encode latency for the 95th percentile: (\d+) us$`)

// analyzeFPS analyzes FPS info from log file and sets as the perf metric.
func analyzeFPS(p *perf.Values, name string, logPath string) error {
	str, err := readFileToString(logPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file: %s", logPath)
	}

	matches := regExpFPS.FindAllStringSubmatch(str, -1)
	if len(matches) != 1 {
		return errors.Errorf("Unexpected occurrence(=%d) of FPS matched pattern. %s", len(matches), str)
	}

	fps, err := strconv.ParseFloat(matches[0][1], 32)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse FPS value: %s", matches[0][1])
	}

	p.Set(perf.Metric{
		Name:      getMetricName(name, keyFPS),
		Unit:      unitFPS,
		Direction: perf.BiggerIsBetter,
	}, fps)
	return nil
}

// analyzeEncodeLatency analyzes encode latency from log file and sets as the perf metrics.
func analyzeEncodeLatency(p *perf.Values, name string, logPath string) error {
	str, err := readFileToString(logPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file: %s", logPath)
	}

	latency50, err := parseOneInt(str, regExpEncodeLatency50)
	if err != nil {
		return errors.Wrap(err, "Failed on parsing encode latency 50")
	}
	latency75, err := parseOneInt(str, regExpEncodeLatency75)
	if err != nil {
		return errors.Wrap(err, "Failed on parsing encode latency 75")
	}
	latency95, err := parseOneInt(str, regExpEncodeLatency95)
	if err != nil {
		return errors.Wrap(err, "Failed on parsing encode latency 95")
	}

	p.Set(perf.Metric{
		Name:      getMetricName(name, keyEncodeLatency50),
		Unit:      unitMicroSecond,
		Direction: perf.SmallerIsBetter,
	}, float64(latency50))
	p.Set(perf.Metric{
		Name:      getMetricName(name, keyEncodeLatency75),
		Unit:      unitMicroSecond,
		Direction: perf.SmallerIsBetter,
	}, float64(latency75))
	p.Set(perf.Metric{
		Name:      getMetricName(name, keyEncodeLatency95),
		Unit:      unitMicroSecond,
		Direction: perf.SmallerIsBetter,
	}, float64(latency95))
	return nil
}

// analyzeCPUUsage analyzes CPU usage from log file and sets as the perf metric.
func analyzeCPUUsage(p *perf.Values, name string, logPath string) error {
	str, err := readFileToString(logPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file: %s", logPath)
	}

	vstr := strings.TrimSuffix(str, "\n")
	v, err := strconv.ParseFloat(vstr, 64)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse r value: %s", vstr)
	}

	p.Set(perf.Metric{
		Name:      getMetricName(name, keyCPUUsage),
		Unit:      unitPercent,
		Direction: perf.SmallerIsBetter,
	}, v)
	return nil
}

// analyzeFrameStats analyzes quality from log file which assumes input is YUV420 (for MSE samples per channel), and sets as the perf metrics.
func analyzeFrameStats(p *perf.Values, name string, logPath string) error {
	str, err := readFileToString(logPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file: %s", logPath)
	}

	num := 0
	ssim := map[string]map[string]float64{
		"y":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"u":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"v":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"combined": {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
	}
	psnr := map[string]map[string]float64{
		"y":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"u":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"v":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"combined": {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
	}

	for i, line := range strings.Split(str, "\n") {
		if i == 0 || line == "" {
			// Skip the first CSV header line or blank line.
			continue
		}
		values := strings.Split(line, ",")
		if len(values) != 9 {
			return errors.Errorf("It should contain exactly 9 numbers per line: %s", line)
		}
		// values[0] is frame index which is un-used.
		width, err := strconv.ParseInt(values[1], 10, 32)
		if err != nil {
			return errors.Errorf("Failed to parse int of width: %s", values[1])
		}
		height, err := strconv.ParseInt(values[2], 10, 32)
		if err != nil {
			return errors.Errorf("Failed to parse int of height: %s", values[2])
		}
		ssimY, err := strconv.ParseFloat(values[3], 32)
		if err != nil {
			return errors.Errorf("Failed to parse float of ssim_y: %s", values[3])
		}
		ssimU, err := strconv.ParseFloat(values[4], 32)
		if err != nil {
			return errors.Errorf("Failed to parse float of ssim_u: %s", values[4])
		}
		ssimV, err := strconv.ParseFloat(values[5], 32)
		if err != nil {
			return errors.Errorf("Failed to parse float of ssim_v: %s", values[5])
		}
		mseY, err := strconv.ParseInt(values[6], 10, 32)
		if err != nil {
			return errors.Errorf("Failed to parse int of mse_y: %s", values[6])
		}
		mseU, err := strconv.ParseInt(values[7], 10, 32)
		if err != nil {
			return errors.Errorf("Failed to parse int of mse_u: %s", values[7])
		}
		mseV, err := strconv.ParseInt(values[8], 10, 32)
		if err != nil {
			return errors.Errorf("Failed to parse int of mse_v: %s", values[8])
		}

		num++
		updateQualityMetrics(ssim["y"], ssimY)
		updateQualityMetrics(ssim["u"], ssimU)
		updateQualityMetrics(ssim["v"], ssimV)
		// Weighting of YUV channels for SSIM taken from libvpx.
		updateQualityMetrics(ssim["combined"], 0.8*ssimY+0.1*(ssimU+ssimV))

		// Samples per MSE score assumes YUV420 subsampling.
		updateQualityMetrics(psnr["y"], mseToPSNR(width*height*4/4, 255, mseY))
		updateQualityMetrics(psnr["u"], mseToPSNR(width*height*4/4, 255, mseU))
		updateQualityMetrics(psnr["v"], mseToPSNR(width*height*4/4, 255, mseV))
		updateQualityMetrics(psnr["combined"], mseToPSNR(width*height*6/4, 255, mseY+mseU+mseV))
	}

	if num == 0 {
		return errors.Errorf("Frame statistics are not existed.")
	}

	for _, cha := range []string{"y", "u", "v", "combined"} {
		// Log stats with a key similar to 'quality.ssim.y.max'. For combined stats the channel is omitted ('quality.ssim.max').
		key := "quality.%s"
		if cha != "combined" {
			key += "." + cha
		}
		key += ".%s"
		for stat, value := range ssim[cha] {
			if stat == "sum" {
				// What we actually want is the average.
				p.Set(perf.Metric{
					Name:      getMetricName(name, fmt.Sprintf(key, "ssim", "avg")),
					Unit:      unitSSIM,
					Direction: perf.BiggerIsBetter,
				}, value/float64(num))
			} else { // "max" and "min" cases
				p.Set(perf.Metric{
					Name:      getMetricName(name, fmt.Sprintf(key, "ssim", stat)),
					Unit:      unitSSIM,
					Direction: perf.BiggerIsBetter,
				}, value)
			}
		}
		for stat, value := range psnr[cha] {
			if stat == "sum" {
				// What we actually want is the average.
				p.Set(perf.Metric{
					Name:      getMetricName(name, fmt.Sprintf(key, "psnr", "avg")),
					Unit:      unitPSNR,
					Direction: perf.BiggerIsBetter,
				}, value/float64(num))
			} else { // "max" and "min" cases
				p.Set(perf.Metric{
					Name:      getMetricName(name, fmt.Sprintf(key, "psnr", stat)),
					Unit:      unitPSNR,
					Direction: perf.BiggerIsBetter,
				}, value)
			}
		}
	}
	return nil
}

// updateQualityMetrics updates the channel min/max/sum with a new sample value.
func updateQualityMetrics(channel map[string]float64, value float64) {
	channel["max"] = math.Max(value, channel["max"])
	channel["min"] = math.Min(value, channel["min"])
	channel["sum"] += value
}

// mseToPSNR calculates PSNR from MSE for a frame.
func mseToPSNR(samples int64, peak int64, mse int64) float64 {
	const maxPSNR float64 = 100.0
	// Prevent a divide-by-zero, MSE at 0 is perfect quality (no error).
	if mse == 0 {
		return maxPSNR
	}
	psnr := 10.0 * math.Log10(float64(peak*peak*samples)/float64(mse))
	return math.Min(psnr, maxPSNR)
}

// parseOneInt is the helper function to parse an integer from input string by specified regexp.
func parseOneInt(str string, re *regexp.Regexp) (int64, error) {
	matches := re.FindAllStringSubmatch(str, -1)
	if len(matches) != 1 {
		return -1, errors.Errorf("Unexpected occurrence(=%d) of matched pattern.", len(matches))
	}
	value, err := strconv.ParseInt(matches[0][1], 10, 32)
	if err != nil {
		return -1, errors.Errorf("Failed to parse to int: %s", matches[0][1])
	}
	return value, nil
}

// readFileToString reads the input file as string.
func readFileToString(filePath string) (string, error) {
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// getMetricName wraps the stream name and key into the metric name.
func getMetricName(name string, key string) string {
	// TODO(johnylin@): Remove "tast_" prefix after removing video_VEAPerf in autotest.
	return fmt.Sprintf("tast_%s.%s", name, key)
}

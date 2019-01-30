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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/perf"
)

var regExpFPS = regexp.MustCompile(`(?m)^Measured encoder FPS: ([+\-]?[0-9.]+)$`)
var regExpEncodeLatency50 = regexp.MustCompile(`(?m)^Encode latency for the 50th percentile: (\d+) us$`)
var regExpEncodeLatency75 = regexp.MustCompile(`(?m)^Encode latency for the 75th percentile: (\d+) us$`)
var regExpEncodeLatency95 = regexp.MustCompile(`(?m)^Encode latency for the 95th percentile: (\d+) us$`)

// reportFPS reports FPS info from log file and sets as the perf metric.
func reportFPS(p *perf.Values, name, logPath string) error {
	const (
		keyFPS  = "fps"
		unitFPS = "fps"
	)

	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %s", logPath)
	}

	matches := regExpFPS.FindAllStringSubmatch(string(b), -1)
	if len(matches) != 1 {
		return errors.Errorf("unexpected occurrence(=%d) of FPS matched pattern: %s", len(matches), string(b))
	}

	fps, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse FPS value: %s", matches[0][1])
	}

	p.Set(perf.Metric{
		Name:      getMetricName(name, keyFPS),
		Unit:      unitFPS,
		Direction: perf.BiggerIsBetter,
	}, fps)
	return nil
}

// reportEncodeLatency reports encode latency from log file and sets as the perf metrics.
func reportEncodeLatency(p *perf.Values, name, logPath string) error {
	const (
		keyEncodeLatency50 = "encode_latency.50_percentile"
		keyEncodeLatency75 = "encode_latency.75_percentile"
		keyEncodeLatency95 = "encode_latency.95_percentile"
		unitMicroSecond    = "us"
	)

	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %s", logPath)
	}
	str := string(b)

	latency50, err := parseOneInt(str, regExpEncodeLatency50)
	if err != nil {
		return errors.Wrap(err, "failed on parsing encode latency 50")
	}
	latency75, err := parseOneInt(str, regExpEncodeLatency75)
	if err != nil {
		return errors.Wrap(err, "failed on parsing encode latency 75")
	}
	latency95, err := parseOneInt(str, regExpEncodeLatency95)
	if err != nil {
		return errors.Wrap(err, "failed on parsing encode latency 95")
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

// reportCPUUsage reports CPU usage from log file and sets as the perf metric.
func reportCPUUsage(p *perf.Values, name, logPath string) error {
	const (
		keyCPUUsage = "cpu_usage"
		unitPercent = "precent"
	)

	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %s", logPath)
	}

	vstr := strings.TrimSuffix(string(b), "\n")
	v, err := strconv.ParseFloat(vstr, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse r value: %s", vstr)
	}

	p.Set(perf.Metric{
		Name:      getMetricName(name, keyCPUUsage),
		Unit:      unitPercent,
		Direction: perf.SmallerIsBetter,
	}, v)
	return nil
}

// reportFrameStats reports quality from log file which assumes input is YUV420 (for MSE samples per channel), and sets as the perf metrics.
func reportFrameStats(p *perf.Values, name, logPath string) error {
	const (
		unitSSIM = "ssim"
		unitPSNR = "psnr"
	)

	b, err := ioutil.ReadFile(logPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %s", logPath)
	}

	num := 0
	ssim := newFrameStats()
	psnr := newFrameStats()

	for i, line := range strings.Split(string(b), "\n") {
		if i == 0 || line == "" {
			// Skip the first CSV header line or blank line.
			continue
		}
		values := strings.Split(line, ",")
		if len(values) != 9 {
			return errors.Errorf("line %d does not contain 9 comma-separated values: %s", i, line)
		}

		var index, width, height, ssimY, ssimU, ssimV, mseY, mseU, mseV float64
		for j, dst := range []*float64{&index, &width, &height, &ssimY, &ssimU, &ssimV, &mseY, &mseU, &mseV} {
			if *dst, err = strconv.ParseFloat(values[j], 64); err != nil {
				return errors.Wrapf(err, "failed to parse %q in field %d", values[j], j)
			}
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
		return errors.New("frame statistics do not exist")
	}

	for _, cha := range []string{"y", "u", "v", "combined"} {
		for stat, value := range ssim[cha] {
			if stat == "sum" {
				// What we actually want is the average.
				value /= float64(num)
			}
			p.Set(perf.Metric{
				Name:      getFrameStatsMetricName(name, cha, "ssim", stat),
				Unit:      unitSSIM,
				Direction: perf.BiggerIsBetter,
			}, value)
		}
		for stat, value := range psnr[cha] {
			if stat == "sum" {
				// What we actually want is the average.
				value /= float64(num)
			}
			p.Set(perf.Metric{
				Name:      getFrameStatsMetricName(name, cha, "psnr", stat),
				Unit:      unitPSNR,
				Direction: perf.BiggerIsBetter,
			}, value)
		}
	}
	return nil
}

// newFrameStats returns a two-dimensional frame stats map with default values.
func newFrameStats() map[string]map[string]float64 {
	return map[string]map[string]float64{
		"y":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"u":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"v":        {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
		"combined": {"max": 0.0, "min": math.MaxFloat64, "sum": 0.0},
	}
}

// getFrameStatsMetricName wraps the input strings into the metric name.
// cha, typ, and stat will be wrapped into the key name such like 'quality.ssim.y.max'. For "combined" stat, cha (channel) is omitted such as 'quality.ssim.max'.
func getFrameStatsMetricName(name, cha, typ, stat string) string {
	if stat == "sum" {
		stat = "avg"
	}

	if cha == "combined" {
		return getMetricName(name, fmt.Sprintf("quality.%s.%s", typ, stat))
	}
	return getMetricName(name, fmt.Sprintf("quality.%s.%s.%s", typ, cha, stat))
}

// updateQualityMetrics updates the channel min/max/sum with a new sample value.
func updateQualityMetrics(channel map[string]float64, value float64) {
	channel["max"] = math.Max(value, channel["max"])
	channel["min"] = math.Min(value, channel["min"])
	channel["sum"] += value
}

// mseToPSNR calculates PSNR from MSE for a frame.
func mseToPSNR(samples, peak, mse float64) float64 {
	const maxPSNR = 100.0
	// Prevent a divide-by-zero, MSE at 0 is perfect quality (no error).
	if mse == 0 {
		return maxPSNR
	}
	psnr := 10.0 * math.Log10(peak*peak*samples/mse)
	return math.Min(psnr, maxPSNR)
}

// parseOneInt is the helper function to parse exactly one integer from input string by specified regexp.
// The regexp result is required to contain exactly one group  matching an integer.
func parseOneInt(str string, re *regexp.Regexp) (int, error) {
	match := re.FindStringSubmatch(str)
	if match == nil {
		return -1, errors.Errorf("%q not found", re)
	}
	return strconv.Atoi(match[1])
}

// getMetricName wraps the stream name and key into the metric name.
// For example, name should contain both stream and codec name such like "tulip2-1280x720_h264", key is the metric name such like "fps".
func getMetricName(name, key string) string {
	// TODO(johnylin@): Remove "tast_" prefix after removing video_VEAPerf in autotest.
	return fmt.Sprintf("tast_%s.%s", name, key)
}

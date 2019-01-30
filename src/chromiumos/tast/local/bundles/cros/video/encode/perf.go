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
		return errors.Errorf("found %d FPS matches in %q; want 1", len(matches), b)
	}

	fps, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse FPS value %q", matches[0][1])
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
		return errors.Wrapf(err, "failed to read file %q", logPath)
	}

	// Iterate over different latency measurements, extracting and reporting each.
	for _, st := range []struct {
		key string         // metric key
		re  *regexp.Regexp // matches latency stat
	}{
		{keyEncodeLatency50, regExpEncodeLatency50},
		{keyEncodeLatency75, regExpEncodeLatency75},
		{keyEncodeLatency95, regExpEncodeLatency95},
	} {
		match := st.re.FindStringSubmatch(string(b))
		if match == nil {
			return errors.Errorf("didn't find match for latency %q in %q", st.re, b)
		}
		val, err := strconv.Atoi(match[1])
		if err != nil {
			return errors.Wrapf(err, "failed converting %q latency %q", st.key)
		}
		p.Set(perf.Metric{
			Name:      getMetricName(name, st.key),
			Unit:      unitMicroSecond,
			Direction: perf.SmallerIsBetter,
		}, float64(val))
	}

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

type channelStats struct {
	min, max, sum float64
	num           int
}

func (cs *channelStats) report(p *perf.Values, prefix, unit string) {
	setPerf := func(stat string, val float64) {
		p.Set(perf.Metric{
			Name:      prefix + stat,
			Unit:      unit,
			Direction: perf.BiggerIsBetter,
		}, val)
	}
	setPerf("min", cs.min)
	setPerf("max", cs.max)
	setPerf("avg", cs.sum/float64(cs.num))
}

type frameStats struct{ y, u, v, combined channelStats }

func (fs *frameStats) report(p *perf.Values, name, typ, unit string) {
	// For "y", "u", "v" channels, the metric key is formatted as "quality.<typ>.<channel>.<stat>", such as "quality.ssim.y.max".
	// For "combined" channel, the metric key is formatted as "quality.<typ>.<stat>", such as "quality.psnr.avg".
	prefix := getMetricName(name, fmt.Sprintf("quality.%s.", typ))
	fs.y.report(p, prefix+"y.", unit)
	fs.u.report(p, prefix+"u.", unit)
	fs.v.report(p, prefix+"v.", unit)
	fs.combined.report(p, prefix, unit)
}

// newFrameStats returns a frameStats struct with default values.
func newFrameStats() frameStats {
	return frameStats{
		y:        channelStats{min: math.MaxFloat64, max: 0.0, sum: 0.0, num: 0},
		u:        channelStats{min: math.MaxFloat64, max: 0.0, sum: 0.0, num: 0},
		v:        channelStats{min: math.MaxFloat64, max: 0.0, sum: 0.0, num: 0},
		combined: channelStats{min: math.MaxFloat64, max: 0.0, sum: 0.0, num: 0},
	}
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

		updateQualityMetrics(&ssim.y, ssimY)
		updateQualityMetrics(&ssim.u, ssimU)
		updateQualityMetrics(&ssim.v, ssimV)
		// Weighting of YUV channels for SSIM taken from libvpx.
		updateQualityMetrics(&ssim.combined, 0.8*ssimY+0.1*(ssimU+ssimV))

		// Samples per MSE score assumes YUV420 subsampling.
		updateQualityMetrics(&psnr.y, mseToPSNR(width*height*4/4, 255, mseY))
		updateQualityMetrics(&psnr.u, mseToPSNR(width*height*4/4, 255, mseU))
		updateQualityMetrics(&psnr.v, mseToPSNR(width*height*4/4, 255, mseV))
		updateQualityMetrics(&psnr.combined, mseToPSNR(width*height*6/4, 255, mseY+mseU+mseV))
	}

	if ssim.y.num == 0 {
		return errors.New("frame statistics do not exist")
	}

	ssim.report(p, name, "ssim", unitSSIM)
	psnr.report(p, name, "psnr", unitPSNR)
	return nil
}

// updateQualityMetrics updates the channel min/max/sum/num with a new sample value.
func updateQualityMetrics(cha *channelStats, value float64) {
	cha.min = math.Min(value, cha.min)
	cha.max = math.Max(value, cha.max)
	cha.sum += value
	cha.num++
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

// getMetricName wraps the stream name and key into the metric name.
// For example, name should contain both stream and codec name such like "tulip2-1280x720_h264", key is the metric name such like "fps".
func getMetricName(name, key string) string {
	// TODO(johnylin@): Remove "tast_" prefix after removing video_VEAPerf in autotest.
	return fmt.Sprintf("tast_%s.%s", name, key)
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package perf

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/cpu"

	"chromiumos/tast/common/perf"
)

// CPUUsageSource is an implementation of perf.TimelineDataSource which reports
// the CPU usage.
type CPUUsageSource struct {
	name            string
	prevStats       map[string]cpu.TimesStat
	maxFreqReported map[string]bool
}

func cpuUtilization(prev, time cpu.TimesStat) float64 {
	prevBusy := prev.User + prev.System + prev.Nice + prev.Iowait + prev.Irq + prev.Softirq + prev.Steal
	newBusy := time.User + time.System + time.Nice + time.Iowait + time.Irq + time.Softirq + time.Steal
	prevTotal := prevBusy + prev.Idle
	newTotal := newBusy + time.Idle
	if prevBusy > newBusy {
		return 0
	}
	if prevTotal >= newTotal {
		return 100
	}
	return math.Max(0, math.Min(100, 100*(newBusy-prevBusy)/(newTotal-prevTotal)))
}

func cpuFreq(cpuName, freqType string) (float64, error) {
	data, err := ioutil.ReadFile(filepath.Join(
		"/sys/devices/system/cpu", cpuName, "cpufreq", fmt.Sprintf("scaling_%s_freq", freqType)))
	if err != nil {
		return 0, err
	}

	freq, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, err
	}
	// frequency data is in kHz.  Converting to MHz.
	return float64(freq) / 1000, nil
}

// NewCPUUsageSource creates a new instance of CPUUsageSource for the given
// metric name.
func NewCPUUsageSource(name string) *CPUUsageSource {
	if name == "" {
		name = "CPUUsage"
	}
	return &CPUUsageSource{name: name, prevStats: map[string]cpu.TimesStat{}, maxFreqReported: map[string]bool{}}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *CPUUsageSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *CPUUsageSource) Start(ctx context.Context) error {
	times, err := cpu.TimesWithContext(ctx, true /*perCPU*/)
	if err != nil {
		return err
	}
	for _, time := range times {
		s.prevStats[time.CPU] = time
	}
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *CPUUsageSource) Snapshot(ctx context.Context, values *perf.Values) error {
	times, err := cpu.TimesWithContext(ctx, true /*perCPU*/)
	if err != nil {
		return err
	}

	var totalPercent float64
	for _, time := range times {
		var percent float64
		var prevTime cpu.TimesStat
		if pt, ok := s.prevStats[time.CPU]; ok {
			prevTime = pt
		} else {
			prevTime = cpu.TimesStat{}
		}
		percent = cpuUtilization(prevTime, time)
		values.Append(perf.Metric{
			Name:      s.name + "." + time.CPU,
			Variant:   "usage",
			Multiple:  true,
			Unit:      "percent",
			Direction: perf.SmallerIsBetter,
		}, percent)
		totalPercent += percent
		s.prevStats[time.CPU] = time
		freq, err := cpuFreq(time.CPU, "cur")
		if err != nil {
			return err
		}
		values.Append(perf.Metric{
			Name:      s.name + "." + time.CPU + ".Frequency",
			Multiple:  true,
			Unit:      "MHz",
			Direction: perf.BiggerIsBetter,
		}, freq)
		if !s.maxFreqReported[time.CPU] {
			maxFreq, err := cpuFreq(time.CPU, "max")
			if err != nil {
				return err
			}
			values.Set(perf.Metric{
				Name:      s.name + "." + time.CPU + ".MaxFrequency",
				Unit:      "MHz",
				Direction: perf.BiggerIsBetter,
			}, maxFreq)
			s.maxFreqReported[time.CPU] = true
		}
	}

	values.Append(perf.Metric{
		Name:      s.name,
		Multiple:  true,
		Unit:      "percent",
		Direction: perf.SmallerIsBetter,
	}, totalPercent/float64(len(times)))

	return nil
}

// Stop does nothing.
func (s *CPUUsageSource) Stop(_ context.Context, values *perf.Values) error {
	return nil
}

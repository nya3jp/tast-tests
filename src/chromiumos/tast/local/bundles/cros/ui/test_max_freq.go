// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cuj

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/testing"
)

// To test if cpuinfo_max_freq changes during test
type CPUMaxFreqSource struct {
	name string
}

func newCPUMaxFreqSource() *CPUMaxFreqSource {
	return &CPUMaxFreqSource{name: "CPUMaxFreq"}
}

// Setup implements perf.TimelineDatasource.Setup.
func (s *CPUMaxFreqSource) Setup(ctx context.Context, prefix string) error {
	s.name = prefix + s.name
	return nil
}

// Start implements perf.TimelineDatasource.Start.
func (s *CPUMaxFreqSource) Start(ctx context.Context) error {
	return nil
}

// Snapshot implements perf.TimelineDatasource.Snapshot.
func (s *CPUMaxFreqSource) Snapshot(ctx context.Context, values *perf.Values) error {
	maxFreqBytes, err := ioutil.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	if err != nil {
		return nil
	}
	maxFreq, err := strconv.ParseFloat(strings.TrimRight(string(maxFreqBytes), "\r\n"), 64)
	if err != nil {
		testing.ContextLog(ctx, err)
		return err
	}
	// testing.ContextLog(ctx, "maxfreq:", maxFreq)
	values.Append(perf.Metric{
		Name:      s.name,
		Multiple:  true,
		Unit:      "Hz",
		Direction: perf.SmallerIsBetter,
	}, maxFreq)
	return nil
}

// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Perf,
		Desc:     "Demonstrates how to emit perf metrics",
		Contacts: []string{"nya@chromium.org", "tast-users@chromium.org"},
	})
}

func Perf(ctx context.Context, s *testing.State) {
	// In order to upload metrics, they should be whitelisted in:
	// src/third_party/autotest/files/tko/perf_upload/perf_dashboard_config.json
	// Example metrics below are not whitelisted, thus ignored.
	var (
		bootTime    = perf.Metric{Name: "example_boot_time", Unit: "ms", Direction: perf.SmallerIsBetter}
		refreshRate = perf.Metric{Name: "example_refresh_rate", Unit: "fps", Direction: perf.BiggerIsBetter, Multiple: true}
	)

	p := perf.NewValues()

	// Single-valued data series can be recorded with Set.
	p.Set(bootTime, 2.83)

	// Multi-valued data series can be recorded with Append.
	p.Append(refreshRate, 60)
	p.Append(refreshRate, 30, 72, 11.5)

	// Variant can be also specified.
	for _, name := range []string{"a.h264", "a.vp8"} {
		cpuUsage := perf.Metric{Name: "example_cpu_usage", Variant: name, Unit: "percent", Direction: perf.SmallerIsBetter}
		p.Set(cpuUsage, 50.0)
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

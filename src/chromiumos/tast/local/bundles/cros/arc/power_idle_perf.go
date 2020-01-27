// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/ectool"
	"chromiumos/tast/local/power/powercap"
	"chromiumos/tast/local/setup"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PowerIdlePerf,
		Desc: "Measures the battery drain of an idle system",
		Contacts: []string{
			"cwd@chromium.org",
			"arcvm-eng@google.com",
		},
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome", "android_both"},
		Params: []testing.Param{{
			Name: "",
		}, {
			Name: "noarc",
			Pre:  chrome.LoggedIn(),
		}, {
			Name:              "arc",
			Pre:               arc.Booted(),
			ExtraSoftwareDeps: []string{"android"},
		}, {
			Name:              "arcvm",
			Pre:               arc.VMBooted(),
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: 15 * time.Minute,
	})
}

func PowerIdlePerf(ctx context.Context, s *testing.State) {
	setup := setup.NewSetup()
	power.DefaultPowerSetup(ctx, setup)
	cleanup := setup.Setup(s)
	defer cleanup(s)

	p := perf.NewValues()
	mb := perf.NewTimelineBuilder()
	mb.Append(powercap.NewRaplMetrics())
	mb.Append(ectool.NewBatteryMetrics(ctx, 2.0))
	metrics, err := mb.Build()
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}
	s.Log("Finished setup")

	const warmupDuration = 120 * time.Second
	if err := testing.Sleep(ctx, warmupDuration); err != nil {
		s.Fatal("Failed to sleep during warmup: ", err)
	}
	if err := metrics.Start(); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}
	const iterationCount = 30
	const iterationDuration = 10 * time.Second
	for i := 0; i < iterationCount; i++ {
		if err := testing.Sleep(ctx, iterationDuration); err != nil {
			s.Fatal("Failed to sleep between metric snapshots: ", err)
		}
		s.Logf("Iteration %d snapshot", i)
		if err := metrics.Snapshot(p); err != nil {
			s.Fatal("Failed to snapshot metrics: ", err)
		}
	}

	if err := p.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/power"
	"chromiumos/tast/local/power/ectool"
	"chromiumos/tast/local/power/powercap"
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
		Vars:    []string{"perf"},
		Timeout: 15 * time.Minute,
	})
}

func PowerIdlePerf(ctx context.Context, s *testing.State) {
	setup := power.NewSetup()
	setup.Append(power.DisableService(ctx, "powerd"))
	setup.Append(power.DisableService(ctx, "update-engine"))
	setup.Append(power.DisableService(ctx, "vnc"))
	setup.Append(power.DisableService(ctx, "dptf"))
	setup.Append(power.SetBacklightLux(ctx, 150))
	setup.Append(power.SetKeyboardBrightness(ctx, 24))
	setup.Append(power.MuteAudio(ctx))
	setup.Append(power.DisableWiFiInterfaces(ctx))
	setup.Append(power.SetBatteryDischarge(ctx, 2.0))
	// TODO: bluetooth
	// TODO: SetLightbarBrightness
	// TODO: nightlight off
	if perf, ok := s.Var("perf"); ok {
		setup.Append(power.PerfTrace(ctx, s.OutDir(), strings.Split(perf, " ")))
	}
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

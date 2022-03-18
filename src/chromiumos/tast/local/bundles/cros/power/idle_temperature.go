// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package power

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/cpu"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IdleTemperature,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Collects data on the idle temperature of devices",
		Contacts:     []string{"edcourtney@chromium.org"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Timeout:      10 * time.Minute,
	})
}

func IdleTemperature(ctx context.Context, s *testing.State) {
	thermal := power.NewSysfsThermalMetrics()
	thermal.Setup(ctx, "")

	// First wait for CPU idle. This is the same timeout WaitUntilCoolDown uses by default.
	testing.Sleep(ctx, 300*time.Second)

	// Grab the temperature.
	t, _, err := cpu.Temperature(ctx)
	if err != nil {
		s.Fatal("Could not get CPU temperature: ", err)
	}

	// Report the temperature.
	pv := perf.NewValues()
	defer func() {
		if err := pv.Save(s.OutDir()); err != nil {
			// Fatal because this is the whole point of the test.
			s.Fatal("Failed to save perf data: ", err)
		}
	}()

	err = thermal.Snapshot(ctx, pv)
	if err != nil {
		s.Fatal("Could not get all thermal zones temperature: ", err)
	}

	pv.Set(perf.Metric{
		Name:      "idle_temperature",
		Unit:      "deg_C",
		Direction: perf.SmallerIsBetter,
	}, float64(t))
}

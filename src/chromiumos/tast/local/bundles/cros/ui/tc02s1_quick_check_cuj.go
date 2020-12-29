// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/local/bundles/cros/ui/cuj"
	"chromiumos/tast/local/bundles/cros/ui/quickcheckcuj"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC02S1QuickCheckCUJ,
		Desc:         "Measures the smoothess of screen unlock and open an gmail thread",
		Contacts:     []string{"xliu@cienet.com", "hc.tsai@cienet.com", "alfredyu@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Fixture:      "loggedInToCUJUserKeepState",
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.cuj_wifissid",
			"ui.cuj_wifipassword",
			"perf_level",
		},
	})
}

// TC02S1QuickCheckCUJ measures the smoothess of screen unlock and open an gmail thread
func TC02S1QuickCheckCUJ(ctx context.Context, s *testing.State) {
	tabletMode := false
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch, err := quickcheckcuj.WakeUpDuration(ctx, s)
	if err != nil {
		s.Fatal("Failed to detect wakeup event: ", err)
	}

	cr := s.FixtValue().(cuj.FixtureData).Chrome

	pv := quickcheckcuj.Run(ctx, s, cr, func(ctx context.Context) error {
		s.Log("Suspending DUT")
		cmd := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--timeout=30", "--wakeup_timeout=60")
		if err := cmd.Run(); err != nil {
			return err
		}

		cr := s.FixtValue().(cuj.FixtureData).Chrome
		return cr.ResetSession(ctx)
		return nil
	}, false, tabletMode)

	select {
	case d := <-ch:
		pv.Set(perf.Metric{
			Name:      "QuickCheckCUJ.WakeUpTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(d.Milliseconds()))
	default:
		s.Error("Failed to find wake up time: ", err)
	}

	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

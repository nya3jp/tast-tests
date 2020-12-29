// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
		Contacts:     []string{"xliu@cienet.com", "hc.tsai@cienet.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome", "arc", "tablet_mode", "wifi"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Pre:          cuj.LoginKeepState(),
		Timeout:      10 * time.Minute,
		Vars: []string{
			"ui.cuj_username",
			"ui.cuj_password",
			"ui.cuj_wifissid",
			"ui.cuj_wifipassword",
		},
	})
}

func TC02S1QuickCheckCUJ(ctx context.Context, s *testing.State) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch, err := quickcheckcuj.WakeUpDuration(ctx)
	if err != nil {
		s.Fatal("Failed to detect wakeup event: ", err)
	}

	pv := quickcheckcuj.Run(ctx, s, func(ctx context.Context) error {
		s.Log("Suspending DUT")
		cmd := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--timeout=30", "--wakeup_timeout=60")
		if err := cmd.Run(); err != nil {
			return err
		}

		cr := s.PreValue().(cuj.PreKeepData).Chrome
		return cr.ResetSession(ctx)
	})

	select {
	case d := <-ch:
		pv.Set(perf.Metric{
			Name:      "QuickCheckCUJ.WakeUpTime",
			Unit:      "ms",
			Direction: perf.SmallerIsBetter,
		}, float64(d.Milliseconds()))
	case <-time.After(10 * time.Second):
		s.Error("Failed to find wake up time")
	}

	if err = pv.Save(s.OutDir()); err != nil {
		s.Error("Failed saving perf data: ", err)
	}
}

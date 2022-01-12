// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/uidetection"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RobloxLaunch,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Captures launch metrics for Roblox",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"roblox-launch-screen-sign-up-button.png"},
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 10 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func RobloxLaunch(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.roblox.client"
		appActivity = ".startup.ActivitySplash"
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// Make sure Roblox is launched.
		uda := uidetection.NewDefault(params.TestConn)

		// Make sure Roblox is launched.
		if err := uiauto.Combine("Confirm launch",
			uda.WithTimeout(time.Minute*3).WaitUntilExists(uidetection.CustomIcon(s.DataPath("roblox-launch-screen-sign-up-button.png"))),
		)(ctx); err != nil {
			return errors.Wrap(err, "failed to confirm launch")
		}

		// Save the metric in crosbolt.
		loadTime := time.Now().Sub(params.ActivityStartTime)
		perfValues := perf.NewValues()
		perfValues.Set(testutil.LaunchTimePerfMetric(), loadTime.Seconds())

		if err := perfValues.Save(s.OutDir()); err != nil {
			return errors.Wrap(err, "failed to save performance values")
		}

		return nil
	})
}

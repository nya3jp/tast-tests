// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/fixtures"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AmongusLaunch,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Captures launch metrics for Among Us",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model(testutil.ModelsToTest()...)),
		Fixture:      fixtures.ARCAppGamePerfFixture,
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			}},
		Timeout: 15 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func AmongusLaunch(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.innersloth.spacemafia"
		appActivity = ".EosUnityPlayerActivity"
	)

	testutil.PerformTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) error {
		// Information about the version will appear when the application is loaded.
		if err := params.Arc.WaitForLogcat(ctx, arc.RegexpPred(regexp.MustCompile(`Among Us Version.*Pipeline Build Num\:.*`))); err != nil {
			return errors.Wrap(err, "version information indicating launch was not found in LogCat")
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

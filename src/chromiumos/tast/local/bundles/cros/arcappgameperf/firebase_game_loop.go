// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FirebaseGameLoop,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "TBD",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		// TODO(b/219524888): Disabled while CAPTCHA prevents test from completing.
		//Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.Model(testutil.ModelsToTest()...)),
		Params: []testing.Param{
			{
				ExtraSoftwareDeps: []string{"android_p"},
				Pre:               pre.ArcAppGamePerfBooted,
			}, {
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
				Pre:               pre.ArcAppGamePerfBooted,
			}},
		Timeout: 15 * time.Minute,
		VarDeps: []string{"arcappgameperf.username", "arcappgameperf.password"},
	})
}

func FirebaseGameLoop(ctx context.Context, s *testing.State) {
	const (
		apkName      = "ArcFirebaseGameLoopTest.apk"
		appName      = "org.chromium.arc.testapp.arcfirebasegamelooptest"
		activityName = ".MainActivity"
	)

	testutil.PerformGameLoopTest(ctx, s, apkName, appName, activityName)
}

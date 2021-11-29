// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arcappgameperf

import (
	"context"
	"time"

	"chromiumos/tast/common/android/ui"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/pre"
	"chromiumos/tast/local/bundles/cros/arcappgameperf/testutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MinecraftEducationEditionLaunch,
		Desc:         "Captures launch metrics for Minecraft Education Edition",
		Contacts:     []string{"davidwelling@google.com", "arc-engprod@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
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

func MinecraftEducationEditionLaunch(ctx context.Context, s *testing.State) {
	const (
		appPkgName  = "com.mojang.minecraftedu"
		appActivity = "com.mojang.minecraftpe.MainActivity"
	)

	testutil.PerformLaunchTest(ctx, s, appPkgName, appActivity, func(params testutil.TestParams) (isLaunched bool, err error) {
		// Minecraft shows a "Sign in to Minecraft" prompt within a webview when it is fully loaded.
		if err := params.Device.Object(ui.ClassName("android.webkit.WebView"), ui.TextContains("Sign in to Minecraft")).WaitForExists(ctx, time.Minute*5); err != nil {
			return false, errors.Wrap(err, "sign in webview was not found")
		}

		return true, nil
	})
}

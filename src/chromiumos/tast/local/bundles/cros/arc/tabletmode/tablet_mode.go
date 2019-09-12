// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package tabletmode implements the sceanrio of tests which requires to be run in freerom windowing mode.
package tabletmode

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

// TestFunc contains the contents of the test itself and is called when the environment and  test app are ready.
type TestFunc func(*arc.ARC, *arc.Activity, *chrome.Conn, *chrome.Chrome)

// RunTest starts Chrome and specified app and this ensures that tablet mode is specified mode when f is executed.
func RunTest(ctx context.Context, s *testing.State, pkgName, clsName, apk string, tabletMode bool, f TestFunc) {
	cr := s.PreValue().(arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	// Be nice and restore tablet mode to its original state on exit.
	defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)

	if tabletModeEnabled != tabletMode {
		if err := ash.SetTabletModeEnabled(ctx, tconn, tabletMode); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(ricardoq): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	a := s.PreValue().(arc.PreData).ARC

	if len(apk) > 0 {
		if err := a.Install(ctx, s.DataPath(apk)); err != nil {
			s.Fatal("Failed installing app: ", err)
		}
	}

	act, err := arc.NewActivity(a, pkgName, clsName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	// This is an issue to re-enable the tablet mode at the end of the test when
	// there is a freeform app still open. See: https://crbug.com/1002666
	defer act.Stop(ctx)

	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	f(a, act, tconn, cr)
}

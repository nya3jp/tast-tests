// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/android/ui"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/display"
	chromeui "chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/mouse"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/power"
	"chromiumos/tast/testing"
)

type arcPIPEnergyAndPowerTestParams struct {
	activityName string
	bigPIP       bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PIPEnergyAndPower,
		Desc:         "Measures energy and power usage of ARC++ PIP",
		Contacts:     []string{"amusbach@chromium.org", "chromeos-wmp@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_nightly"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Fixture:      "arcBooted",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "small",
			Val:  arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: false},
		}, {
			Name: "big",
			Val:  arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivity", bigPIP: true},
		}, {
			Name: "small_blend",
			Val:  arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: false},
		}, {
			Name: "big_blend",
			Val:  arcPIPEnergyAndPowerTestParams{activityName: ".VideoActivityWithRedSquare", bigPIP: true},
		}},
	})
}

func PIPEnergyAndPower(ctx context.Context, s *testing.State) {
	// Reserve one minute for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, time.Minute)
	defer cancel()

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to ensure clamshell mode: ", err)
	}
	defer cleanup(cleanupCtx)

	if err := ash.HideVisibleNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to hide notifications: ", err)
	}

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath("ArcPipVideoTest.apk")); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}

	displayMode, err := info.GetSelectedMode()
	if err != nil {
		s.Fatal("Failed to get the selected display mode of the primary display: ", err)
	}

	kw, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard event writer: ", err)
	}
	defer kw.Close()

	timeline, err := perf.NewTimeline(ctx, power.TestMetrics())
	if err != nil {
		s.Fatal("Failed to build metrics: ", err)
	}

	if _, err := power.WaitUntilCPUCoolDown(ctx, power.CoolDownPreserveUI); err != nil {
		s.Fatal("Failed to wait for CPU to cool down: ", err)
	}

	params := s.Param().(arcPIPEnergyAndPowerTestParams)
	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.pictureinpicturevideo", params.activityName)
	if err != nil {
		s.Fatal("Failed to create activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start app: ", err)
	}
	defer act.Stop(cleanupCtx, tconn)

	// The test activity enters PIP mode in onUserLeaveHint().
	if err := act.SetWindowState(ctx, tconn, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to minimize app: ", err)
	}

	var pipWindow *ash.Window
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		pipWindow, err = ash.FindWindow(ctx, tconn, func(w *ash.Window) bool { return w.State == ash.WindowStatePIP })
		if err != nil {
			return errors.Wrap(err, "the PIP window hasn't been created yet")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to wait for PIP window: ", err)
	}

	if params.bigPIP {
		if err := mouse.Move(ctx, tconn, pipWindow.TargetBounds.CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse to PIP window: ", err)
		}

		// The PIP resize handle is an ImageView with no android:contentDescription.
		// Here we use the regex (?!.+) to match the empty content description. See:
		// frameworks/base/packages/SystemUI/res/layout/pip_menu_activity.xml
		resizeHandleBounds, err := d.Object(
			ui.ClassName("android.widget.ImageView"),
			ui.DescriptionMatches("(?!.+)"),
			ui.PackageName("com.android.systemui"),
		).GetBounds(ctx)
		if err != nil {
			s.Fatal("Failed to get bounds of PIP resize handle: ", err)
		}

		if err := mouse.Move(ctx, tconn, coords.ConvertBoundsFromPXToDP(resizeHandleBounds, displayMode.DeviceScaleFactor).CenterPoint(), time.Second); err != nil {
			s.Fatal("Failed to move mouse to PIP resize handle: ", err)
		}
		if err := mouse.Press(ctx, tconn, mouse.LeftButton); err != nil {
			s.Fatal("Failed to press left mouse button: ", err)
		}
		if err := mouse.Move(ctx, tconn, info.WorkArea.TopLeft(), time.Second); err != nil {
			if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
				s.Fatal("Failed to move mouse for dragging PIP resize handle, and then failed to release left mouse button: ", err)
			}
			s.Fatal("Failed to move mouse for dragging PIP resize handle: ", err)
		}
		if err := mouse.Release(ctx, tconn, mouse.LeftButton); err != nil {
			s.Fatal("Failed to release left mouse button: ", err)
		}

		if err := chromeui.WaitForLocationChangeCompleted(ctx, tconn); err != nil {
			s.Fatal("Failed to wait for location-change events to be propagated to the automation API: ", err)
		}

		pipWindow, err = ash.GetWindow(ctx, tconn, pipWindow.ID)
		if err != nil {
			s.Fatal("PIP window gone after resize: ", err)
		}

		if 5*pipWindow.TargetBounds.Width <= 2*info.WorkArea.Width && 5*pipWindow.TargetBounds.Height <= 2*info.WorkArea.Height {
			s.Fatalf("Expected big PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	} else {
		if 10*pipWindow.TargetBounds.Width >= 3*info.WorkArea.Width && 10*pipWindow.TargetBounds.Height >= 3*info.WorkArea.Height {
			s.Fatalf("Expected small PIP window. Got a %v PIP window in a %v work area", pipWindow.TargetBounds.Size(), info.WorkArea.Size())
		}
	}

	conn, err := cr.NewConn(ctx, "chrome://settings")
	if err != nil {
		s.Fatal("Failed to load chrome://settings: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for chrome://settings to achieve quiescence: ", err)
	}

	// Tab away from the search box of chrome://settings, so that
	// there will be no blinking cursor.
	if err := kw.Accel(ctx, "Tab"); err != nil {
		s.Fatal("Failed to send Tab: ", err)
	}

	// triedToStopTracing means that cr.StopTracing(cleanupCtx)
	// was already done, with or without success (if it failed
	// then we have no reason to try again with the same timeout).
	triedToStopTracing := false
	defer func() {
		if triedToStopTracing {
			return
		}
		if _, err := cr.StopTracing(cleanupCtx); err != nil {
			s.Error("Failed to stop tracing viz.triangles in cleanup phase: ", err)
		}
	}()
	if err := cr.StartTracing(ctx, []string{"disabled-by-default-viz.triangles"}); err != nil {
		s.Fatal("Failed to start tracing viz.triangles: ", err)
	}

	if err := timeline.Start(ctx); err != nil {
		s.Fatal("Failed to start metrics: ", err)
	}

	if err := timeline.StartRecording(ctx); err != nil {
		s.Fatal("Failed to start recording: ", err)
	}

	const timelineDuration = time.Minute
	if err := testing.Sleep(ctx, timelineDuration); err != nil {
		s.Fatalf("Failed to wait %v: %v", timelineDuration, err)
	}

	pv, err := timeline.StopRecording()
	if err != nil {
		s.Fatal("Error while recording metrics: ", err)
	}

	// As we still have to save results to files, we are not yet
	// focusing on cleanup, but we can safely pass cleanupCtx
	// (borrowing from the time reserved for cleanup) because
	// StopTracing was deferred to cleanup and we are now getting
	// it done ahead of time (see comment on triedToStopTracing).
	triedToStopTracing = true
	tr, err := cr.StopTracing(cleanupCtx)
	if err != nil {
		s.Fatal("Failed to stop tracing viz.triangles: ", err)
	}

	if err := pv.Save(s.OutDir()); err != nil {
		s.Error("Failed to save perf data: ", err)
	}

	if err := chrome.SaveTraceToFile(ctx, tr, filepath.Join(s.OutDir(), "trace.data.gz")); err != nil {
		s.Error("Failed to save trace data: ", err)
	}
}
